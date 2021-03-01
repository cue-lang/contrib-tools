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
	"encoding/json"
	"flag"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v31/github"
)

var fUpdate = flag.Bool("update", false, "whether to update golden files")

func TestPayloads(t *testing.T) {
	must := func(dro github.DispatchRequestOptions, err error) github.DispatchRequestOptions {
		if err != nil {
			t.Fatalf("failed to build payload: %v", err)
		}
		return dro
	}
	testCases := map[string]github.DispatchRequestOptions{
		"runtrybot": must(buildRunTryBotPayload(clTriggerPayload{
			ChangeID: "change",
			Ref:      "ref",
			Commit:   "commit",
		})),
		"mirror": must(buildMirrorPayload("hello")),
		"importpr": must(buildImportPRPayload("hello", importPRPayload{
			PR: 123,
		})),
		"unity_versions": must(buildUnityPayload("hello", unityPayload{
			Versions: "\"v0.3.0-beta.5\"",
		})),
		"unity_cl": must(buildUnityPayloadFromCLTrigger(clTriggerPayload{
			ChangeID: "change",
			Ref:      "ref",
			Commit:   "commit",
		})),
	}

	for key, dro := range testCases {
		t.Run(key, func(t *testing.T) {
			byts, err := json.MarshalIndent(dro, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}
			fn := filepath.Join("testdata", key+".golden")
			golden, err := ioutil.ReadFile(fn)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v", fn, err)
			}
			if !cmp.Equal(byts, golden) {
				if !*fUpdate {
					t.Fatalf("output did not match golden file:\n%s", cmp.Diff(byts, golden))
				}
				if err := ioutil.WriteFile(fn, byts, 0666); err != nil {
					t.Fatalf("failed to update golden file %v: %v", fn, err)
				}
			}
		})
	}
}
