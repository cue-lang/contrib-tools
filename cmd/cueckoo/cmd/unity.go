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
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v53/github"
	"github.com/spf13/cobra"
)

const (
	flagUnityVersions flagName = "versions"
)

// newUnityCmd creates a new unity command
func newUnityCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unity",
		Short: "run unity against a version of CUE",
		Long: `
Usage of unity:

	unity [--normal] [ARGS...]

When run with no arguments, unity derives a revision and change ID for each
pending commit in the current branch. If multiple pending commits are found,
you must either specify which commits to run, or specify HEAD to run the
unity for all of them.

If the --normal flag is provided, then the list of arguments is interpreted as
versions understood by unity.

unity needs your GitHub username and a personal acccess token with the "repo" scope.
You can configure them via your git credential helper, or by setting GITHUB_USER and GITHUB_PAT.
Note that the personal access token should be "classic"; GitHub's new
fine-grained tokens are still in beta and haven't been tested to work here.
`,
		RunE: mkRunE(c, unityDef),
	}
	cmd.Flags().Bool(string(flagUnityVersions), false, "pass arguments to unity as versions")
	return cmd
}

func unityDef(cmd *Command, args []string) error {
	cfg, err := loadConfig(cmd.Context())
	if err != nil {
		return err
	}

	// If we are passed --normal, interpret all args as versions to be passed to
	// unity
	if flagUnityVersions.Bool(cmd) {
		unquoted := strings.Join(args, " ")
		for i, a := range args {
			args[i] = strconv.Quote(a)
		}
		var up unityPayload
		up.Type = string(eventTypeUnity)
		up.Versions = strings.Join(args, " ")
		payload, err := buildUnityPayload(fmt.Sprintf("unity run for versions %s", unquoted), up)
		if err != nil {
			return err
		}
		return cfg.triggerRepositoryDispatch(cfg.unityOwner, cfg.unityRepo, payload)
	}

	// Interpret as a request to test CLs

	r := newCLTrigger(cmd, cfg, func(payload repositoryDispatchPayload) error {
		payload.Type = string(eventTypeUnity)
		p, err := buildUnityPayloadFromCLTrigger(payload)
		if err != nil {
			return err
		}
		if err := cfg.triggerRepositoryDispatch(cfg.unityOwner, cfg.unityRepo, p); err != nil {
			return err
		}
		return nil
	})
	return r.run()
}

type unityPayload struct {
	repositoryDispatchPayload

	// Versions is string "list" of versions against
	// which to run unity, e.g.
	//
	//    "\"v0.3.0-beta.5\" \"v0.3.0-beta.4\""
	//
	Versions string `json:"versions,omitempty"`
}

func buildUnityPayload(msg string, payload unityPayload) (github.DispatchRequestOptions, error) {
	return buildDispatchPayload(msg, payload)
}

func buildUnityPayloadFromCLTrigger(payload repositoryDispatchPayload) (github.DispatchRequestOptions, error) {
	msg := fmt.Sprintf("unity run for %v", payload.Ref)
	return buildDispatchPayload(msg, unityPayload{
		repositoryDispatchPayload: payload,
	})
}
