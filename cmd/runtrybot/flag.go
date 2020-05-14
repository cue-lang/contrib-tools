package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	flagSet = flag.NewFlagSet("runtrybot", flag.ContinueOnError)
	fChange = flagSet.Bool("change", false, "interpret arguments as change numbers or IDs")
)

func init() { flagSet.Usage = usage }

func usage() {
	fmt.Fprintf(os.Stderr, `
Usage of runtrybot:

	runtrybot [-change] [ARGS...]

When run with no arguments, runtrybot derives a revision and change ID for each
pending commit in the current branch. If multiple pending commits are found,
you must either specify which commits to run, or specify HEAD to run the
trybots for all of them.

If the -change flag is provide, then the list of arguments is interpreted as
change numbers or IDs, and the latest revision from each of those changes is
assumed.

runtrybot requires GITHUB_USER and GITHUB_PAT environment variables to be set
with your GitHub username and personal acccess token respectively. The personal
access token only requires "public_repo" scope.

Flags:

`[1:])
	flagSet.PrintDefaults()
}

type usageErr string

func (u usageErr) Error() string { return string(u) }

type flagErr string

func (f flagErr) Error() string { return string(f) }

func main() { os.Exit(main1()) }

func main1() int {
	err := mainerr()
	if err == nil {
		return 0
	}
	switch err.(type) {
	case usageErr:
		fmt.Fprintln(os.Stderr, err)
		flagSet.Usage()
		return 2
	case flagErr:
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}
