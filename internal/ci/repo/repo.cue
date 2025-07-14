// package repo contains data values that are common to all CUE configurations
// in this repo. The list of configurations includes GitHub workflows, but also
// things like gerrit configuration etc.
package repo

import (
	"github.com/cue-lang/contrib-tools/internal/ci/base"
)

base

githubRepositoryPath: "cue-lang/contrib-tools"

botGitHubUser:      "cueckoo"
botGitHubUserEmail: "cueckoo@gmail.com"

linuxMachine: "ns-linux-amd64"

latestGo: "1.24.x"
