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

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	cliToken        = kingpin.Flag("token", "GitHub API token").Envar("GITHUB_TOKEN").Required().String()
	cliOrg          = kingpin.Flag("org", "Organisation members to sync").Required().String()
	cliTeam         = kingpin.Flag("team", "Comma-separated list of team within organsation to sync").String()
	cliFile         = kingpin.Flag("file", "Authorized keys file to write to.").Required().String()
	cliOwner        = kingpin.Flag("owner", "Enforce this owner").Required().String()
	cliDaemon       = kingpin.Flag("daemon", "Runs in daemon mode").Bool()
	cliDaemonPeriod = kingpin.Flag("daemon-period", "Time (in minutes) that key is updated").Default("5").Int()
)

func main() {
	kingpin.Parse()

	gh := github.NewClient(oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: *cliToken,
		},
	)))

	ctx := context.Background()

	if *cliDaemon == true {
		log.Println("Running in daemon mode")
		period := time.Duration(*cliDaemonPeriod) * time.Minute
		for {
			syncKeys(ctx, gh)
			time.Sleep(period)
		}
	} else {
		syncKeys(ctx, gh)
	}
}

func syncKeys(ctx context.Context, gh *github.Client) {
	// Retrieve the members of specified organisation.
	members, _, err := gh.Organizations.ListMembers(ctx, *cliOrg, &github.ListMembersOptions{})
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
	} else {
		for _, member := range members {
			membersFiltered = append(membersFiltered, *member)
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

// GetTeamByName looks up a team by name.
func GetTeamByName(ctx context.Context, gh *github.Client, org, name string) (*github.Team, error) {
	teams, _, err := gh.Organizations.ListTeams(ctx, org, &github.ListOptions{})
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

// UserInTeam returns boolean value indicating user is an active member of a team.
func UserInTeam(ctx context.Context, gh *github.Client, user *github.User, team *github.Team) bool {
	membership, _, _ := gh.Organizations.GetTeamMembership(ctx, team.GetID(), user.GetLogin())
	if membership.GetState() == "active" {
		return true
	}

	return false
}

// GetUserSSHKeys returns ssh public keys associated with the specified user.
func GetUserSSHKeys(ctx context.Context, gh *github.Client, user github.User) ([]Key, error) {
	userKeys := []Key{}
	ghKeys, _, err := gh.Users.ListKeys(ctx, user.GetLogin(), &github.ListOptions{})
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
