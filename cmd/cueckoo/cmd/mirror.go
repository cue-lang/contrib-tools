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
	"github.com/google/go-github/v31/github"
	"github.com/spf13/cobra"
)

// newMirrorCmd creates a new mirror command
func newMirrorCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Synchronise Gerrit with the GitHub mirror ",
		RunE:  mkRunE(c, mirrorDef),
	}
	return cmd
}

func mirrorDef(c *Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	msg := "Mirror Gerrit to GitHub"
	payload, err := buildMirrorPayload(msg)
	if err != nil {
		return err
	}
	err = cfg.triggerRepositoryDispatch(payload)
	if err != nil {
		return err
	}
	return nil
}

func buildMirrorPayload(msg string) (github.DispatchRequestOptions, error) {
	return buildDispatchPayload(msg, eventTypeMirror, nil)
}
