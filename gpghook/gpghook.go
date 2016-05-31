// gpghook implements a GitHub webhook that verifies all commits on a branch are
// signed by a GPG key currently trusted in the master branch.
// This is most useful if you use option #1 of
// https://mikegerwitz.com/papers/git-horror-story#merge-1, i.e. squash all
// branches on merge to master. This webhook should be considered as a
// convenience to protect against accidentally pushing an unsigned or
// incorrectly signed commit. Each developer and build machine should have code
// that checks locally for correct signatures on each pull.
// Also note that signing all commits means you cannot use GitHub's web-based
// merge feature.

// The set of trusted keys is represented by a GPG keyring in the `keyring`
// subdirectory of the repository. To add keys to it, run:
// GNUPGHOME=/path/to/repository/keyring gpg --import -a key.asc
// Then edit keyring/gpg.conf and add:
// trusted-key <long key id>

// Credentials for the GitHub API are read from ~/.netrc, like so:
// machine api.github.com login foo password <Personal access token>
// This is convenient because `curl -n` and Python's requests library both
// support the same format. You should *not* use your GitHub password here.
// Get a personal access token from https://github.com/settings/tokens and give
// it repo:status scope.

// This program expects to be run in a git directory, with a GitHub remote named
// `origin`, with Git > 1.7.9 and GPG2 installed.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

func postWrap(w http.ResponseWriter, r *http.Request) {
	err := post(w, r)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), 500)
		return
	}
}

var gitMu sync.Mutex

func post(w http.ResponseWriter, r *http.Request) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	var content struct {
		After string
	}
	err = json.Unmarshal(body, &content)
	if err != nil {
		return err
	}
	// Check that "after" is a valid sha1 string
	after, err := hex.DecodeString(content.After)
	if err != nil {
		return err
	}
	if len(after) != 20 {
		return fmt.Errorf("Wrong length for 'after' field: %d", len(after))
	}
	err = setState("pending", content.After)
	if err != nil {
		return fmt.Errorf("setState: %s", err)
	}
	gitMu.Lock()
	defer gitMu.Unlock()
	log.Printf("Checking signature on %s", content.After)
	err = exec.Command("git", "fetch", "origin").Run()
	if err != nil {
		return fmt.Errorf("fetch: %s", err)
	}
	err = exec.Command("git", "checkout", "origin/master").Run()
	if err != nil {
		return fmt.Errorf("checkout: %s", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Setenv("GNUPGHOME", cwd+"/keyring")
	result, err := exec.Command("git", "log", "--pretty=%G?", "master.."+content.After).Output()
	if err != nil {
		return fmt.Errorf("git log: %s", err)
	}
	if len(result) == 0 {
		return fmt.Errorf("No results")
	}
	for _, line := range strings.Split(string(result), "\n") {
		if string(line) != "G" && string(line) != "" {
			return fmt.Errorf("Commit not signed right: '%s'", string(line))
		}
	}
	log.Printf("Success for %s", content.After)
	return setState("success", content.After)
}

var client http.Client

func setState(state, sha string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/statuses/%s", *repo, sha)
	body := fmt.Sprintf(`{"state": "%s", "context": "gpghook"}`, state)
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(user, password)
	client.Do(req)
	return nil
}

var user string
var password string

func loadCredentials() error {
	netRe := regexp.MustCompile("machine\\s+api.github.com\\s+login\\s+(?P<user>.*)\\s+password\\s+(?P<password>.*)")
	netrc, err := ioutil.ReadFile(os.Getenv("HOME") + "/.netrc")
	if err != nil {
		return err
	}
	match := netRe.FindStringSubmatch(string(netrc))
	if match != nil {
		user = match[1]
		password = match[2]
		return nil
	}
	return fmt.Errorf("No match for netrc pattern found")
}

var listen = flag.String("listen", ":8000", "Host/port to listen on")
var repo = flag.String("repo", "jsha/playground", "Repo to monitor")

func main() {
	flag.Parse()
	err := exec.Command("git", "status").Run()
	if err != nil {
		log.Fatal("Must be run from a git directory")
	}
	err = loadCredentials()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", postWrap)
	http.ListenAndServe(*listen, nil)
}
