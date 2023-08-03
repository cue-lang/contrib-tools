// Copyright 2021 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/andygrunwald/go-gerrit"
	"github.com/cue-sh/tools/internal/codereviewcfg"
	"github.com/google/go-github/v53/github"
)

// eventType values define an enumeration of the various
// GitHub repository dispatch workflows that can be triggered
// by cueckoo
type eventType string

const (
	// NOTE: the values for trybot and unity must be consistent with the names
	// defined in the cuelang.org/go/internal/ci/base package.
	//
	// TODO: refactor to sort out types.
	eventTypeTrybot   eventType = "trybot"
	eventTypeImportPR eventType = "importpr"
	eventTypeUnity    eventType = "unity"
)

// config holds the configuration that is loaded from the codereview config
// found within the root of the git directory that contains the working
// directory. Put another way, cueckoo needs to be run from within the main
// cue repo.
type config struct {
	// gerritURL is the URL of the Gerrit instance
	gerritURL string

	// githubURL is the URL for the GitHub repo
	githubURL string

	// githubOwner is the organisation/user to which the GitHub repo belongs
	githubOwner string

	// githubRepo is the name of the GitHub repo
	githubRepo string

	// unityOwner is the organisation/user to which the unity repo belongs
	unityOwner string

	// unityRepo is the name of the unity repo
	unityRepo string

	// githubClient is the client for using the GitHub API
	githubClient *github.Client

	// gerritClient is the client for using the Gerrit API
	gerritClient *gerrit.Client
}

// loadConfig loads the repository configuration from codereview.cfg, using
// gh as the key to find the relevant GitHub information
func loadConfig(ctx context.Context) (*config, error) {
	var res config

	// Determine git root directory. Note it will have trailing newline
	gitRoot, err := run(ctx, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("failed to determine git root: %w", err)
	}

	cfg, err := codereviewcfg.Config(strings.TrimSpace(gitRoot))
	if err != nil {
		return nil, fmt.Errorf("failed to load codereview config: %v", err)
	}

	gerritURL := cfg["gerrit"]
	if gerritURL == "" {
		return nil, fmt.Errorf("missing Gerrit server in codereview config")
	}
	res.gerritURL, err = codereviewcfg.GerritURLToServer(gerritURL)
	if err != nil {
		return nil, fmt.Errorf("failed to derived Gerrit server from %v: %v", gerritURL, err)
	}

	githubURL := cfg["github"]
	if githubURL == "" {
		return nil, fmt.Errorf("missing GitHub repo in codereview config")
	}
	res.githubURL = githubURL
	res.githubOwner, res.githubRepo, err = codereviewcfg.GithubURLToParts(githubURL)
	if err != nil {
		return nil, fmt.Errorf("failed to derive GitHub owner and repo from %v: %v", githubURL, err)
	}

	// Unity configuration is optional.
	// We check the "new" config entry first, as we transition to a single Unity entry in cue-lang/cue again.
	for _, entry := range []string{"cue-unity-new", "cue-unity"} {
		if unityURL := cfg[entry]; unityURL != "" {
			res.unityOwner, res.unityRepo, err = codereviewcfg.GithubURLToParts(unityURL)
			if err != nil {
				return nil, fmt.Errorf("failed to derive unity owner and repo from %v: %v", unityURL, err)
			}
			break
		}
	}

	githubUser, githubPassword, err := gitCredentials(ctx, githubURL)
	if githubUser == "" || githubPassword == "" || err != nil {
		// Fall back to the manual env vars.
		githubUser = os.Getenv("GITHUB_USER")
		githubPassword = os.Getenv("GITHUB_PAT")
		if githubUser == "" || githubPassword == "" {
			return nil, fmt.Errorf("configure a git credential helper or set GITHUB_USER and GITHUB_PAT")
		}
	}
	githubAuth := github.BasicAuthTransport{Username: githubUser, Password: githubPassword}
	res.githubClient = github.NewClient(githubAuth.Client())

	gerritUser, gerritPassword, err := gitCredentials(ctx, gerritURL)
	if gerritUser == "" || gerritPassword == "" || err != nil {
		// Fall back to the manual env vars.
		gerritUser = os.Getenv("GERRIT_USER")
		gerritPassword = os.Getenv("GERRIT_PASSWORD")
		if gerritUser == "" || gerritPassword == "" {
			return nil, fmt.Errorf("configure a git credential helper or set GERRIT_USER and GERRIT_PASSWORD")
		}
	}
	res.gerritClient, err = gerrit.NewClient(res.gerritURL, nil)
	if err != nil {
		return nil, err
	}
	res.gerritClient.Authentication.SetBasicAuth(gerritUser, gerritPassword)

	return &res, nil
}

func gitCredentials(ctx context.Context, repoURL string) (username, password string, _ error) {
	// For example:
	//
	//    $ git credential fill
	//    protocol=https
	//    host=example.com
	//    path=foo.git
	//    ^D
	//    protocol=https
	//    host=example.com
	//    username=bob
	//    password=secr3t

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	input := strings.Join([]string{
		"protocol=" + u.Scheme,
		"host=" + u.Host,
		"path=" + u.Path,
	}, "\n") + "\n" // `git credential` wants a trailing newline
	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Stdin = strings.NewReader(input)
	outputBytes, err := cmd.Output()
	if err != nil {
		if err, _ := err.(*exec.ExitError); err != nil {
			// stderr was captured by Output
			return "", "", fmt.Errorf("failed to run %q: %v:\n%s", cmd.Args, err, err.Stderr)
		}
		return "", "", fmt.Errorf("failed to run %q: %v", cmd.Args, err)
	}
	for _, line := range strings.Split(string(outputBytes), "\n") {
		if line == "" {
			continue // ignore the trailing empty line
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return "", "", fmt.Errorf("invalid output line: %q", line)
		}
		switch key {
		case "protocol", "host", "path":
			// input keys are repeated; ignore them.
		case "username":
			username = val
		case "password":
			password = val
		default:
			// Could happen if the user configured an auth mechanism we don't support, like oauth.
			return "", "", fmt.Errorf("unknown output line key: %q", line)
		}
	}
	return username, password, nil
}

func (c *config) triggerRepositoryDispatch(owner, repo string, payload github.DispatchRequestOptions) error {
	debugf("triggerRepositoryDispatch in %s/%s with payload:\n%s\n", owner, repo, payload.ClientPayload)
	_, resp, err := c.githubClient.Repositories.Dispatch(context.Background(), owner, repo, payload)
	if err != nil {
		return fmt.Errorf("failed to send dispatch event: %v", err)
	}
	if resp.StatusCode/100 != 2 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte("(failed to read body)")
		}
		return fmt.Errorf("dispatch call did not succeed; status code %v\n%s", resp.StatusCode, body)
	}
	return nil
}

func buildDispatchPayload(msg string, payload interface{}) (ro github.DispatchRequestOptions, err error) {
	byts, err := json.Marshal(payload)
	if err != nil {
		return ro, fmt.Errorf("failed to marshal payload: %v", err)
	}
	rm := json.RawMessage(byts)

	ro.EventType = msg
	ro.ClientPayload = &rm

	return
}
