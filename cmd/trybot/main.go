// trybot is a GitHub Action for updating Gerrit CLs with build status
// information. This package is a thin wrapper around:
//
//    github.com/cue-sh/tools/cmd/trybot/trybot
package main

import "github.com/cue-sh/tools/cmd/trybot/trybot"

func main() {
	trybot.Main()
}
