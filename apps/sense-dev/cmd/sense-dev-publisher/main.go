package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
)

func main() {
	data := flag.String("data", "", "controller state directory (publisher user needs read/write)")
	decision := flag.String("decision", "", "live independent Release Gate decision ID")
	repo := flag.String("repo", "", "trusted local repository with approved commit")
	token := flag.String("token-file", "", "private repository-scoped GitHub token file")
	askpass := flag.String("askpass", "", "trusted Git askpass executable for branch push")
	flag.Parse()
	if *data == "" || *decision == "" || *token == "" {
		log.Fatal("data, decision and token-file are required")
	}
	store, err := core.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := store.RunPublish(ctx, *decision, publisher.GitHub{RepoPath: *repo, TokenFile: *token, Askpass: *askpass})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "%s %s %s\n", result.ID, result.Status, result.ExternalID)
}
