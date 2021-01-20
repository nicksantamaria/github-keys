GitHub: Authorized Keys
========================

[![CircleCI](https://circleci.com/gh/nicksantamaria/github-keys.svg?style=svg)](https://circleci.com/gh/nicksantamaria/github-keys)

**Maintainer**: Nick Santamaria

A simple `authorized_keys` file generator backed by GitHub organisations & teams.

## Usage

Sync keys for all users within a team.

```
export GITHUB_TOKEN=xxxx
github-keys \
  --org org-name \
  --team team-name \
  --file ~/.ssh/authorized_keys \
  --owner $(whoami)
```

Sync keys for all users with access to a repo.

```
export GITHUB_TOKEN=xxxx
github-keys \
  --org org-name \
  --repo repo-name \
  --file ~/.ssh/authorized_keys \
  --owner $(whoami)
```

## How it works

* Writes a new `authorized_keys` file
* Ensures permissions are correct
