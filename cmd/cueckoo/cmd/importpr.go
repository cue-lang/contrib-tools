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

	"github.com/google/go-github/v31/github"
	"github.com/spf13/cobra"
)

// newImportPRCmd creates a new importpr command
func newImportPRCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "importpr",
		Short: "Import GitHub PRs to Gerrit",
		RunE:  mkRunE(c, importPRDef),
	}
	return cmd
}

func importPRDef(c *Command, args []string) error {
	cfg, err := loadConfig(targetGitHub)
	if err != nil {
		return err
	}

	if len(args) != 1 {
		return fmt.Errorf("expected a single PR number")
	}

	pr, err := strconv.Atoi(args[0])

	if err != nil || pr <= 0 {
		return fmt.Errorf("%q is not a valid number", pr)
	}

	// TODO: validate that the number provided is indeed a valid PR

	// TODO: work out support for multiple PRs

	msg := fmt.Sprintf("Import PR %d from GitHub to Gerrit", pr)
	payload, err := buildImportPRPayload(msg, importPRPayload{
		PR: pr,
	})
	if err != nil {
		return err
	}
	err = cfg.triggerRepositoryDispatch(payload)
	if err != nil {
		return err
	}
	return nil
}

type importPRPayload struct {
	PR int `json:"pr"`
}

func buildImportPRPayload(msg string, payload importPRPayload) (github.DispatchRequestOptions, error) {
	return buildDispatchPayload(msg, eventTypeImportPR, payload)
}
