package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// taskCmd は `kensan task <list|move|state>` を処理する。
// API ハンドラと同じ internal/tasks のロジックを通る（書き込みロジックの一元化）。
func taskCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kensan task <list|today|move|state> [flags]")
	}
	root, err := dataDir()
	if err != nil {
		return err
	}
	ws := workspace.New(root)

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("task list", flag.ExitOnError)
		jsonOut := fs.Bool("json", false, "JSON で出力")
		_ = fs.Parse(args[1:])
		board, err := tasks.Collect(root)
		if err != nil {
			return err
		}
		if *jsonOut {
			return json.NewEncoder(os.Stdout).Encode(board)
		}
		printBoard(board)
		return nil

	case "today":
		fs := flag.NewFlagSet("task today", flag.ExitOnError)
		file := fs.String("file", "", "対象ファイル（workspace 相対）")
		line := fs.Int("line", 0, "行番号（1-based）")
		text := fs.String("text", "", "タスクのテキスト（一致確認用）")
		on := fs.Bool("on", true, "@today を付ける(true)/外す(false)")
		_ = fs.Parse(args[1:])
		if *file == "" || *line < 1 || *text == "" {
			return fmt.Errorf("-file, -line, -text は必須")
		}
		updated, err := tasks.SetToday(ws, *file, *line, *text, *on)
		if err != nil {
			return err
		}
		fmt.Printf("%s:%d [%s] → today=%v\n", updated.File, updated.Line, updated.Display, updated.Today)
		return nil

	case "move":
		// 完了タスクの daily 退避（/reflection 相当）。今日やる ⇄ ストックは today サブコマンド。
		fs := flag.NewFlagSet("task move", flag.ExitOnError)
		file := fs.String("file", "", "移動元ファイル（workspace 相対）")
		line := fs.Int("line", 0, "移動元の行番号（1-based）")
		text := fs.String("text", "", "タスクのテキスト（行内容の一致確認用）")
		date := fs.String("date", "", "日付 YYYY-MM-DD（省略時は今日）")
		_ = fs.Parse(args[1:])
		if *file == "" || *line < 1 || *text == "" {
			return fmt.Errorf("-file, -line, -text は必須")
		}
		dest := tasks.Dest{Kind: "daily"}
		if *date != "" {
			t, err := time.Parse("2006-01-02", *date)
			if err != nil {
				return fmt.Errorf("invalid date: %s", *date)
			}
			dest.Date = t
		}
		moved, err := tasks.Move(ws, *file, *line, *text, dest)
		if err != nil {
			return err
		}
		fmt.Printf("moved → %s:%d [%s]\n", moved.File, moved.Line, moved.Text)
		return nil

	case "state":
		fs := flag.NewFlagSet("task state", flag.ExitOnError)
		file := fs.String("file", "", "対象ファイル（workspace 相対）")
		line := fs.Int("line", 0, "行番号（1-based）")
		text := fs.String("text", "", "タスクのテキスト（一致確認用）")
		state := fs.String("state", "", "todo | done | skipped")
		_ = fs.Parse(args[1:])
		if *file == "" || *line < 1 || *text == "" || *state == "" {
			return fmt.Errorf("-file, -line, -text, -state は必須")
		}
		updated, err := tasks.SetState(ws, *file, *line, *text, *state)
		if err != nil {
			return err
		}
		fmt.Printf("%s:%d [%s] → %s\n", updated.File, updated.Line, updated.Text, updated.State)
		return nil

	default:
		return fmt.Errorf("unknown task subcommand: %s", args[0])
	}
}

func printBoard(b tasks.Board) {
	section := func(name string, ts []tasks.Task) {
		if len(ts) == 0 {
			return
		}
		fmt.Printf("\n%s (%d)\n", name, len(ts))
		for _, t := range ts {
			mark := map[string]string{"todo": " ", "done": "x", "skipped": "-"}[t.State]
			txt := t.Display
			if txt == "" {
				txt = t.Text
			}
			meta := t.Project
			if t.Milestone != "" {
				meta = strings.TrimSpace(meta + " ▸ " + t.Milestone)
			}
			loc := fmt.Sprintf("%s:%d", t.File, t.Line)
			if meta != "" {
				fmt.Printf("  [%s] %-40s  (%s)  %s\n", mark, txt, meta, loc)
			} else {
				fmt.Printf("  [%s] %-40s  %s\n", mark, txt, loc)
			}
		}
	}
	section("今日やる", b.Today)
	section("ストック", b.Stock)
	section("いつかやる", b.Someday)
	section("マイルストーン", b.Milestones)
}

func dataDir() (string, error) {
	if root := os.Getenv("KENSAN_DATA_DIR"); root != "" {
		return root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "kensan-workspace"), nil
}
