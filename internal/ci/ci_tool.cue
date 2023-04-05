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

package ci

import (
	"path"

	"encoding/yaml"

	"tool/exec"
	"tool/file"

	"github.com/cue-sh/tools/internal/ci/repo"
	"github.com/cue-sh/tools/internal/ci/github"
)

_goos: string @tag(os,var=os)

// gen.workflows regenerates the GitHub workflow Yaml definitions.
//
// See internal/ci/gen.go for details on how this step fits into the sequence
// of generating our CI workflow definitions, and updating various txtar tests
// with files from that process.
command: gen: {
	_dir: path.FromSlash("../../.github/workflows", path.Unix)

	workflows: {
		remove: {
			glob: file.Glob & {
				glob: path.Join([_dir, "*.yml"], _goos)
				files: [...string]
			}
			for _, _filename in glob.files {
				"delete \(_filename)": file.RemoveAll & {
					path: _filename
				}
			}
		}
		for _workflowName, _workflow in github.workflows {
			let _filename = _workflowName + repo.workflowFileExtension
			"generate \(_filename)": file.Create & {
				$after: [ for v in remove {v}]
				filename: path.Join([_dir, _filename], _goos)
				let donotedit = repo.doNotEditMessage & {#generatedBy: "internal/ci/ci_tool.cue", _}
				contents: "# \(donotedit)\n\n\(yaml.Marshal(_workflow))"
			}
		}
	}
}

command: gen: codereviewcfg: file.Create & {
	_dir:     path.FromSlash("../../", path.Unix)
	filename: path.Join([_dir, "codereview.cfg"], _goos)
	let res = repo.toCodeReviewCfg & {#input: repo.codeReview, _}
	let donotedit = repo.doNotEditMessage & {#generatedBy: "internal/ci/ci_tool.cue", _}
	contents: "# \(donotedit)\n\n\(res)\n"
}

// updateTxtarTests ensures certain txtar tests are updated with the
// relevant files that make up the process of generating our CI
// workflows.
//
// See internal/ci/gen.go for details on how this step fits into the sequence
// of generating our CI workflow definitions, and updating various txtar tests
// with files from that process.
//
// This also explains why the ../../ relative path specification below appear
// wrong in the context of the containing directory internal/ci/vendor.
command: updateTxtarTests: {
	goos: _goos

	readJSONSchema: file.Read & {
		_path:    path.FromSlash("../../cue.mod/pkg/github.com/SchemaStore/schemastore/src/schemas/json/github-workflow.cue", path.Unix)
		filename: path.Join([_path], goos.GOOS)
		contents: string
	}
	cueDefInternalCI: exec.Run & {
		cmd:    "go run cuelang.org/go/cmd/cue def cuelang.org/go/internal/ci"
		stdout: string
	}
	// updateEvalTxtarTest updates the cue/testdata/eval testscript which exercises
	// the evaluation of the workflows defined in internal/ci (which by definition
	// means resolving and using the vendored GitHub Workflow schema)
	updateEvalTxtarTest: {
		_relpath: path.FromSlash("../../cue/testdata/eval/github.txtar", path.Unix)
		_path:    path.Join([_relpath], goos.GOOS)

		githubSchema: exec.Run & {
			stdin: readJSONSchema.contents
			cmd:   "go run cuelang.org/go/internal/ci/updateTxtar - \(_path) cue.mod/pkg/github.com/SchemaStore/schemastore/src/schemas/json/github-workflow.cue"
		}
		defWorkflows: exec.Run & {
			$after: githubSchema
			stdin:  cueDefInternalCI.stdout
			cmd:    "go run cuelang.org/go/internal/ci/updateTxtar - \(_path) workflows.cue"
		}
	}
	// When we have a solution for cuelang.org/issue/709 we can make this a
	// file.Glob
	readToolsFile: file.Read & {
		filename: "ci_tool.cue"
		contents: string
	}
	updateCmdCueCmdTxtarTest: {
		_relpath: path.FromSlash("../../cmd/cue/cmd/testdata/script/cmd_github.txt", path.Unix)
		_path:    path.Join([_relpath], goos.GOOS)

		githubSchema: exec.Run & {
			stdin: readJSONSchema.contents
			cmd:   "go run cuelang.org/go/internal/ci/updateTxtar - \(_path) cue.mod/pkg/github.com/SchemaStore/schemastore/src/schemas/json/github-workflow.cue"
		}
		defWorkflows: exec.Run & {
			$after: githubSchema
			stdin:  cueDefInternalCI.stdout
			cmd:    "go run cuelang.org/go/internal/ci/updateTxtar - \(_path) internal/ci/workflows.cue"
		}
		toolsFile: exec.Run & {
			stdin: readToolsFile.contents
			cmd:   "go run cuelang.org/go/internal/ci/updateTxtar - \(_path) internal/ci/\(readToolsFile.filename)"
		}
	}
}
