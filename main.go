package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	cliToken      = kingpin.Flag("token", "GitHub API token").Envar("GITHUB_TOKEN").Required().String()
	cliOrg        = kingpin.Flag("org", "Organisation members to sync").Required().String()
	cliTeam       = kingpin.Flag("team", "Comma-separated list of team within organsation to sync").String()
	cliRepo       = kingpin.Flag("repo", "Comma-separated list of repositories within organsation to get collaborators").String()
	cliFile       = kingpin.Flag("file", "Authorized keys file to write to.").Required().String()
	cliOwner      = kingpin.Flag("owner", "Enforce this owner").Required().String()
	cliDaemon     = kingpin.Flag("daemon", "Runs in daemon mode").Bool()
	cliSyncPeriod = kingpin.Flag("sync-period", "How often to sync GitHub keys (when daemon mode enabled)").Default("5m").Duration()
)

func main() {
	kingpin.Parse()

	if *cliRepo != "" && *cliTeam != "" {
		fmt.Println("Can not specify both --team and --repo flags")
		os.Exit(1)
	}

	gh := github.NewClient(oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: *cliToken,
		},
	)))

	ctx := context.Background()

	if *cliDaemon == true {
		log.Println("Running in daemon mode")
		limiter := time.Tick(*cliSyncPeriod)
		for {
			syncKeys(ctx, gh)
			<-limiter
		}
	} else {
		syncKeys(ctx, gh)
	}
}

func syncKeys(ctx context.Context, gh *github.Client) {
	// Retrieve the members of specified organisation.
	members, err := ListOrgMembers(ctx, gh, *cliOrg)
	if err != nil {
		log.Println("failed to retrieve org members:", err)
		os.Exit(1)
	}

	// Filter memberlist result if team(s) are specified.
	membersFiltered := []github.User{}
	if *cliTeam != "" {
		teams := strings.Split(*cliTeam, ",")
		for _, team := range teams {
			team, err := GetTeamByName(ctx, gh, *cliOrg, team)
			if err != nil {
				log.Println("failed to filter members by team:", err)
				os.Exit(1)
			}

			for _, member := range members {
				if UserInTeam(ctx, gh, member, team) {
					membersFiltered = append(membersFiltered, *member)
				}
			}
		}
	} else if *cliRepo == "" {
		for _, member := range members {
			membersFiltered = append(membersFiltered, *member)
		}
	}

	// Grab collaborators for a repo if specified.
	if *cliRepo != "" {
		repos := strings.Split(*cliRepo, ",")
		for _, repo := range repos {
			collaborators, err := GetRepoCollaborators(ctx, gh, *cliOrg, repo)
			if err != nil {
				panic(err)
			}

			for _, collaborator := range collaborators {
				membersFiltered = append(membersFiltered, *collaborator)
			}
		}
	}

	// Fetch public keys of filtered members.
	keys := []Key{}
	for _, member := range membersFiltered {
		memberKeys, err := GetUserSSHKeys(ctx, gh, member)
		if err != nil {
			log.Println("failed to fetch public keys:", err)
			os.Exit(1)
		}
		keys = append(keys, memberKeys...)
	}

	// Render authorized_keys and save to disk.
	authorizedKeys := RenderAuthorizedKeys(keys)

	err = ioutil.WriteFile(*cliFile, authorizedKeys, 0600)
	if err != nil {
		log.Println("failed to write authorized file:", err)
		os.Exit(1)
	}

	log.Println("file has been written:", *cliFile)

	userData, err := user.Lookup(*cliOwner)
	if err != nil {
		log.Println("failed to lookup users uid/gid:", err)
		os.Exit(1)
	}

	uid, err := strconv.Atoi(userData.Uid)
	if err != nil {
		log.Println("failed to marshall users uid:", err)
		os.Exit(1)
	}

	gid, err := strconv.Atoi(userData.Gid)
	if err != nil {
		log.Println("failed to marshall users gid:", err)
		os.Exit(1)
	}

	err = os.Chown(*cliFile, uid, gid)
	if err != nil {
		log.Println("failed to chown authorized file:", err)
		os.Exit(1)
	}

	log.Println("user permissions have been updated")
}

// ListOrgMembers returns members of specified organisation.
func ListOrgMembers(ctx context.Context, gh *github.Client, org string) ([]*github.User, error) {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())

	var members []*github.User
	opt := &github.ListMembersOptions{}
	var err error
	for range ticker.C {
		for {
			membersPage, resp, err := gh.Organizations.ListMembers(ctx, org, opt)
			if err != nil {
				log.Println("failed to retrieve org members, retrying...")
				continue
			}
			members = append(members, membersPage...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		ticker.Stop()
		break
	}

	return members, err
}

// GetTeamByName looks up a team by name.
func GetTeamByName(ctx context.Context, gh *github.Client, org, name string) (*github.Team, error) {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())

	var teams []*github.Team
	var err error
	for range ticker.C {
		teams, _, err = gh.Organizations.ListTeams(ctx, org, &github.ListOptions{})
		if err != nil {
			log.Println("failed to retrieve team, retrying...")
			continue
		}

		ticker.Stop()
		break
	}

	if err != nil {
		return nil, err
	}

	for _, team := range teams {
		if team.GetName() == name {
			return team, nil
		}
	}

	return nil, fmt.Errorf("Team %s not part of organisation %s", name, org)
}

// GetRepoCollaborators returns collaborators for a repo.
func GetRepoCollaborators(ctx context.Context, gh *github.Client, orgName, orgRepo string) ([]*github.User, error) {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())

	var users []*github.User
	var err error
	for range ticker.C {
		ListOptions := &github.ListCollaboratorsOptions{}

		users, _, err = gh.Repositories.ListCollaborators(ctx, orgName, orgRepo, ListOptions)
		if err != nil {
			log.Println("failed to retrieve collaborators, retrying...", err)
			continue
		}

		ticker.Stop()
		break
	}

	return users, err
}

// UserInTeam returns boolean value indicating user is an active member of a team.
func UserInTeam(ctx context.Context, gh *github.Client, user *github.User, team *github.Team) bool {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())

	var membership *github.Membership
	var err error

	for range ticker.C {
		membership, _, err = gh.Organizations.GetTeamMembership(ctx, team.GetID(), user.GetLogin())
		if err != nil {
			// 404 response code means the user is not in the team - not an API malfunction.
			if strings.Contains(fmt.Sprintf("%s", err), "404 Not Found") == false {
				log.Printf("failed to retrieve user membership, retrying... (error: %s)", err)
				continue
			}
		}

		ticker.Stop()
		break
	}

	if membership.GetState() == "active" {
		return true
	}

	return false
}

// GetUserSSHKeys returns ssh public keys associated with the specified user.
func GetUserSSHKeys(ctx context.Context, gh *github.Client, user github.User) ([]Key, error) {
	ticker := backoff.NewTicker(backoff.NewExponentialBackOff())

	var ghKeys []*github.Key
	var err error
	userKeys := []Key{}

	for range ticker.C {
		ghKeys, _, err = gh.Users.ListKeys(ctx, user.GetLogin(), &github.ListOptions{})
		if err != nil {
			log.Println("failed to retrieve user ssh keys, retrying...")
			continue
		}

		ticker.Stop()
		break
	}

	for _, item := range ghKeys {
		userKeys = append(userKeys, Key{
			Comment: fmt.Sprintf("%s - %d", user.GetLogin(), item.GetID()),
			Key:     item.GetKey(),
		})
	}

	return userKeys, err
}

// RenderAuthorizedKeys does exactly what it sounds like.
func RenderAuthorizedKeys(keys []Key) []byte {
	var buffer bytes.Buffer
	for _, key := range keys {
		item := fmt.Sprintf("# %s\n%s\n", key.Comment, key.Key)
		buffer.WriteString(item)
	}
	return buffer.Bytes()
}
