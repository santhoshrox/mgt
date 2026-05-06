// Package repo resolves the current git repo to a registered mgt-be repo_id.
// It caches the result under <git-root>/.git/mgt-repo-id to avoid an API
// round-trip on every command.
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh/mgt/pkg/client"
	"github.com/santhosh/mgt/pkg/git"
)

const cacheFile = "mgt-repo-id"

// Resolve looks up the repo (owner/name) for the working tree, returning the
// matching server-side Repo. On the first call after `mgt login`, this hits
// /api/repos and caches the chosen ID under .git/.
func Resolve(c *client.Client) (client.Repo, error) {
	if err := git.EnsureRepo(); err != nil {
		return client.Repo{}, err
	}
	root, err := git.RootPath()
	if err != nil {
		return client.Repo{}, err
	}

	// Cached ID takes priority.
	if id := readCachedID(root); id > 0 {
		repos, err := c.ListRepos()
		if err == nil {
			for _, r := range repos {
				if r.ID == id {
					return r, nil
				}
			}
		}
	}

	owner, name := git.OwnerRepo(git.RemoteURL("origin"))
	if owner == "" || name == "" {
		return client.Repo{}, fmt.Errorf("could not parse owner/repo from `git remote get-url origin`")
	}

	repos, err := c.ListRepos()
	if err != nil {
		return client.Repo{}, err
	}
	for _, r := range repos {
		if strings.EqualFold(r.Owner, owner) && strings.EqualFold(r.Name, name) {
			_ = writeCachedID(root, r.ID)
			return r, nil
		}
	}

	// Not registered yet: trigger a sync and try again.
	if synced, err := c.SyncRepos(); err == nil {
		for _, r := range synced {
			if strings.EqualFold(r.Owner, owner) && strings.EqualFold(r.Name, name) {
				_ = writeCachedID(root, r.ID)
				return r, nil
			}
		}
	}
	return client.Repo{}, fmt.Errorf("%s/%s is not registered with mgt-be (run `mgt sync-repos`)", owner, name)
}

func readCachedID(root string) int64 {
	b, err := os.ReadFile(filepath.Join(root, ".git", cacheFile))
	if err != nil {
		return 0
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return id
}

func writeCachedID(root string, id int64) error {
	return os.WriteFile(filepath.Join(root, ".git", cacheFile), []byte(strconv.FormatInt(id, 10)), 0o644)
}
