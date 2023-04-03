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
	"os"
	"strings"

	"github.com/cue-sh/tools/internal/codereviewcfg"
	"github.com/google/go-github/v31/github"
	"golang.org/x/build/gerrit"
)

// eventType values define an enumeration of the various
// GitHub repository dispatch workflows that can be triggered
// by cueckoo
type eventType string

const (
	eventTypeTrybot   eventType = "trybot"
	eventTypeImportPR eventType = "importpr"
	eventTypeUnity    eventType = "unity"
)

type repositoryDispatchPayload struct {
	Type    eventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

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

	// unity configuration is optional
	unityURL := cfg["cue-unity"]
	if unityURL != "" {
		res.unityOwner, res.unityRepo, err = codereviewcfg.GithubURLToParts(unityURL)
		if err != nil {
			return nil, fmt.Errorf("failed to derive unity owner and repo from %v: %v", unityURL, err)
		}
	}

	var auth github.BasicAuthTransport
	auth.Username, err = mustGetEnv("GITHUB_USER")
	if err != nil {
		return nil, err
	}
	auth.Password, err = mustGetEnv("GITHUB_PAT")
	if err != nil {
		return nil, err
	}
	res.githubClient = github.NewClient(auth.Client())
	res.gerritClient = gerrit.NewClient(res.gerritURL, gerrit.NoAuth)

	return &res, nil
}

func mustGetEnv(name string) (string, error) {
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return val, nil
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

func buildDispatchPayload(msg string, et eventType, payload interface{}) (ro github.DispatchRequestOptions, err error) {
	rp := repositoryDispatchPayload{
		Type:    et,
		Payload: payload,
	}
	byts, err := json.Marshal(rp)
	if err != nil {
		return ro, fmt.Errorf("failed to marshal payload: %v", err)
	}
	rm := json.RawMessage(byts)

	ro.EventType = msg
	ro.ClientPayload = &rm

	return
}
