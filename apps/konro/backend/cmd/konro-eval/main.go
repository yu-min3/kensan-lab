// konro-eval runs Konro's import evaluation cases through agent-runtime and
// records them in Langfuse. See apps/konro/docs/llmops.md.
//
//	konro-eval sync
//	konro-eval run -name baseline [-prompt-file p.txt] [-split dev] [-out r.json]
//	konro-eval compare a.json b.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalcases"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalrun"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: konro-eval sync | run -name NAME | compare A.json B.json")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	switch os.Args[1] {
	case "sync":
		set := mustCases()
		lf := mustLangfuse()
		id, err := evalrun.Sync(ctx, lf, set)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("dataset %s (%s): %d items, cases %s\n", evalrun.DatasetName, id, len(set.Cases), evalcases.Hash())
	case "run":
		run(ctx, os.Args[2:])
	case "compare":
		if len(os.Args) != 4 {
			log.Fatal("usage: konro-eval compare A.json B.json")
		}
		fmt.Print(evalrun.Compare(readReport(os.Args[2]), readReport(os.Args[3])))
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func run(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	name := fs.String("name", "", "experiment name (e.g. baseline, prompt-v2)")
	promptFile := fs.String("prompt-file", "", "candidate instruction used for both modes (default: production prompt)")
	split := fs.String("split", "dev", "dev | holdout | all")
	model := fs.String("model-label", "runtime-default", "operator's model selection; part of the cache key")
	cache := fs.String("cache", defaultCache(), "output cache directory ('' disables)")
	out := fs.String("out", "", "write the JSON report here")
	noLF := fs.Bool("no-langfuse", false, "do not sync or send scores (traces still follow OTEL_* settings)")
	_ = fs.Parse(args)
	if *split == "all" {
		*split = ""
	}
	set := mustCases()
	cases := set.Select(*split)
	if len(cases) == 0 {
		log.Fatal("no cases for split")
	}
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = shutdown(c)
	}()
	agent, err := agentclient.FromEnvironment(ctx)
	if err != nil || agent == nil {
		log.Fatalf("agent-runtime connection required (KONRO_AGENT_*): %v", err)
	}
	cfg := evalrun.Config{Name: *name, ModelLabel: *model, CacheDir: *cache}
	if *promptFile != "" {
		b, err := os.ReadFile(*promptFile)
		if err != nil {
			log.Fatal(err)
		}
		cfg.Prompts = map[string]string{"steps": string(b), "faithful": string(b)}
	}
	var lf evalrun.Langfuse
	if !*noLF {
		l := mustLangfuse()
		if cfg.DatasetID, err = evalrun.Sync(ctx, l, set); err != nil {
			log.Fatal(err)
		}
		lf = l
	}
	rep, err := evalrun.Run(ctx, agent, lf, cases, cfg)
	if *out != "" {
		b, _ := json.MarshalIndent(rep, "", "  ")
		if werr := os.WriteFile(*out, b, 0600); werr != nil {
			log.Print(werr)
		}
	}
	fmt.Printf("%s: %+v\n", rep.ExperimentID, rep.Summary)
	for _, r := range rep.Results {
		fmt.Printf("  %-24s %-8s %-13s %.2f %v\n", r.CaseID, r.Split, r.Status, r.Score, r.Reasons)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func mustCases() evalcases.Set {
	s, err := evalcases.Load()
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func mustLangfuse() *evalrun.HTTPLangfuse {
	lf, err := evalrun.NewHTTPLangfuse(os.Getenv("LANGFUSE_HOST"), os.Getenv("LANGFUSE_PUBLIC_KEY"), os.Getenv("LANGFUSE_SECRET_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	return lf
}

func readReport(path string) evalrun.Report {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var r evalrun.Report
	if err := json.Unmarshal(b, &r); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	return r
}

func defaultCache() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "konro-eval")
}
