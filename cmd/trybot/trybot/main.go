// Package trybot is the implementation package behind github.com/cue-sh/trybot
package trybot

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"cuelang.org/go/pkg/strings"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sethvargo/go-githubactions"
	"golang.org/x/build/gerrit"
	"gopkg.in/retry.v1"
)

const (
	modeStart = "start"
	modeEnd   = "end"
)

func mainerr() (err error) {
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return flagErr(err.Error())
	}
	defer handleKnown(&err)

	mode := githubactions.GetInput("mode")
	dir := githubactions.GetInput("dir")
	maxWait := githubactions.GetInput("maxWait")
	runID := githubactions.GetInput("runID")
	inputRef := githubactions.GetInput("ref")
	changeID := githubactions.GetInput("changeID")
	commit := githubactions.GetInput("commit")
	repo := githubactions.GetInput("repo")
	gerritServer := githubactions.GetInput("gerritServer")
	gerritUser := githubactions.GetInput("gerritUser")
	gerritPassword := githubactions.GetInput("gerritPassword")
	gerritCookie := githubactions.GetInput("gerritCookie")

	if (gerritUser == "" || gerritPassword == "") && gerritCookie == "" {
		return fmt.Errorf("missing credentials; supply either gerritUser and gerritPassword, or gerritCookie")
	}

	var c *gerrit.Client
	if gerritCookie != "" {
		tf, err := ioutil.TempFile("", "")
		if err != nil {
			return fmt.Errorf("failed to create temp file for gitcookie details")
		}
		defer os.Remove(tf.Name())
		_, err = fmt.Fprintf(tf, "%v\n", gerritCookie)
		check(err, "failed to write gitcookie data to %v: %v", tf, err)

		err = tf.Close()
		check(err, "failed to close %v: %v", tf, err)

		c = gerrit.NewClient(gerritServer, gerrit.GitCookieFileAuth(tf.Name()))
	} else {
		c = gerrit.NewClient(gerritServer, gerrit.BasicAuth(gerritUser, gerritPassword))
	}

	switch mode {
	case "start":
		fmt.Printf("Gerrit server: %v, changeID: %v, commit: %v\n", gerritServer, changeID, commit)
		err = c.SetReview(context.Background(), changeID, commit, gerrit.ReviewInput{
			Message: fmt.Sprintf("Started the build... see progress at %v/actions/runs/%v", repo, runID),
		})
		check(err, "failed to update start message: %v", err)
	case "checkout":
		r, err := git.PlainOpen(dir)
		check(err, "failed to find git repository: %v", err)

		wt, err := r.Worktree()
		check(err, "failed to get worktree: %v", err)

		origin, err := r.Remote("origin")
		check(err, "failed to find git remote origin: %v", err)
		wait, err := time.ParseDuration(maxWait)
		check(err, "failed to parse max wait time %q: %v", maxWait, err)
		strategy := retry.LimitTime(wait,
			retry.Exponential{
				Initial: 2 * time.Second,
				Factor:  1.5,
			},
		)

		for a := retry.Start(strategy, nil); a.Next(); {
			err = origin.Fetch(&git.FetchOptions{
				RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("%v:%v", inputRef, inputRef))},
			})
			if err == nil || err == git.NoErrAlreadyUpToDate {
				break
			}
			if !strings.HasPrefix(err.Error(), "couldn't find remote ref") {
				check(err, "failed to find remote ref %v: %v", inputRef, err)
			}
			fmt.Fprintf(os.Stderr, "cound't find remote ref; will retry\n")
		}

		err = wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.ReferenceName(inputRef),
		})
		check(err, "failed to checkout %v: %v", inputRef, err)

		ref, err := r.Head()
		check(err, "failed to get HEAD: %v", err)

		actCommit, err := r.CommitObject(ref.Hash())
		check(err, "failed to derive commit from HEAD: %v", err)

		if act := actCommit.Hash.String(); act != commit {
			raise("actual commit %v for ref %v did not match input commit %v", act, inputRef, commit)
		}
	case "end":
		jobStatus := strings.ToLower(githubactions.GetInput("jobStatus"))
		score := -1
		msg := fmt.Sprintf("Build status: %v", jobStatus)
		if jobStatus == "success" {
			score = 1
		} else {
			matrixDesc := strings.ToLower(githubactions.GetInput("matrixDesc"))
			if matrixDesc != "" {
				msg = fmt.Sprintf("%v; %v", msg, matrixDesc)
			}
		}
		err = c.SetReview(context.Background(), changeID, commit, gerrit.ReviewInput{
			Message: msg,
			Labels: map[string]int{
				"Code-Review": score,
			},
		})
		check(err, "failed to update end message: %v", err)
	}

	return nil
}

type knownError struct{ error }

func handleKnown(err *error) {
	switch r := recover().(type) {
	case nil:
	case knownError:
		*err = r
	default:
		panic(r)
	}
}

func check(err error, format string, args ...interface{}) {
	if err != nil {
		panic(knownError{fmt.Errorf(format, args...)})
	}
}

func raise(format string, args ...interface{}) {
	panic(knownError{fmt.Errorf(format, args...)})
}
