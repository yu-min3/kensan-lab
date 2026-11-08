package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/watchmetrics"
	"os"
	"time"
)

func main() {
	repo := flag.String("repo", "", "public GitHub owner/repository")
	user := flag.String("zenn", "", "Zenn username")
	flag.Parse()
	root := os.Getenv("KENSAN_DATA_DIR")
	if root == "" {
		fmt.Fprintln(os.Stderr, "KENSAN_DATA_DIR is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e := watchmetrics.NewCollector().Refresh(ctx, root, *repo, *user, time.Now()); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("public metric observations updated")
}
