// konro-import is the Phase 0 PoC: convert a Recipe Keeper export zip into
// konro recipe markdown files and report anything that did not fit the
// assumed format (unknown itemprops, missing images, empty fields).
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/rkimport"
)

func main() {
	zipPath := flag.String("zip", "", "Recipe Keeper export zip")
	outDir := flag.String("out", "recipes-out", "output directory for markdown + images")
	flag.Parse()
	if *zipPath == "" {
		fmt.Fprintln(os.Stderr, "usage: konro-import -zip <export.zip> [-out <dir>]")
		os.Exit(2)
	}

	res, images, err := rkimport.ParseZip(*zipPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	date := time.Now().Format("2006-01-02")
	taken := map[string]bool{}
	empty := 0
	if err := os.MkdirAll(filepath.Join(*outDir, "images"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	for _, r := range res.Recipes {
		if len(r.Ingredients) == 0 || len(r.Steps) == 0 {
			empty++
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: ingredients=%d steps=%d", r.Title, len(r.Ingredients), len(r.Steps)))
		}
		name := rkimport.Filename(r.Title, taken)
		md := rkimport.Markdown(r, "images", date)
		if err := os.WriteFile(filepath.Join(*outDir, name), []byte(md), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
	for ref, data := range images {
		if err := os.WriteFile(filepath.Join(*outDir, "images", path.Base(ref)), data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	fmt.Printf("recipes:  %d written to %s (recipes with empty sections: %d)\n", len(res.Recipes), *outDir, empty)
	fmt.Printf("images:   %d\n", len(images))
	if props := res.SortedUnknownProps(); len(props) > 0 {
		fmt.Println("unknown itemprops (format drift — inspect these):")
		for _, p := range props {
			fmt.Println("  -", p)
		}
	}
	if len(res.Warnings) > 0 {
		fmt.Printf("warnings (%d):\n", len(res.Warnings))
		for _, w := range res.Warnings {
			fmt.Println("  -", w)
		}
	}
}
