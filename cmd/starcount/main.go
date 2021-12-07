package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

var (
	fOldRepo = flag.String("old", "cuelang/cue", "old repo")
	fNewRepo = flag.String("new", "cue-lang/cue", "old repo")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_PAT")},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := graphql.NewClient("https://api.github.com/graphql", httpClient)

	oldGazers := make(map[string]bool)
	newGazers := make(map[string]bool)
	eg, _ := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return query(client, *fOldRepo, oldGazers)
	})
	eg.Go(func() error {
		return query(client, *fNewRepo, newGazers)
	})
	if err := eg.Wait(); err != nil {
		log.Fatalf("failed to query gazers: %v", err)
	}
	allGazers := make(map[string]bool)
	for g := range oldGazers {
		allGazers[g] = true
	}
	for g := range newGazers {
		allGazers[g] = true
	}
	fmt.Printf("old stargazers: %v\n", len(oldGazers))
	fmt.Printf("new stargazers: %v\n", len(newGazers))
	fmt.Printf("all stargazers: %v\n", len(allGazers))
}

func query(client *graphql.Client, repo string, gazers map[string]bool) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repo not expected format: %q", repo)
	}
	owner, repo := parts[0], parts[1]
	var after *graphql.String
	for {
		var q stargazersQuery
		args := map[string]interface{}{
			"owner": graphql.String(owner),
			"repo":  graphql.String(repo),
			"after": after,
		}
		if err := client.Query(context.Background(), &q, args); err != nil {
			return fmt.Errorf("query failed: %v", err)
		}
		for _, e := range q.Repository.Stargazers.Edges {
			gazers[string(e.Node.Login)] = true
			after = &e.Cursor
		}
		if !q.Repository.Stargazers.PageInfo.HasNextPage {
			break
		}
	}
	return nil
}

// discussionsQuery is the query that gives us discussions + their comments + the
// comments' replies
type stargazersQuery struct {
	Repository struct {
		ID         graphql.String
		Stargazers struct {
			PageInfo PageInfo
			Edges    []*struct {
				Cursor graphql.String
				Node   struct {
					Login graphql.String
				}
			}
		} `graphql:"stargazers(first:100, after:$after)"`
	} `graphql:"repository(name: $repo, owner: $owner)"`
}

type PageInfo struct {
	HasNextPage graphql.Boolean
}
