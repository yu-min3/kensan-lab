// Package history は workspace 内ファイルの git 履歴への読み取りアクセスを提供する。
//
// 設計: アプリは git に書き込まない（commit は Mac / 通常の作業フローのみ。
// unification-plan.md の「git 操作は Mac のみ、cluster 側は読むだけ」）。
// ここは log / show を読むだけ。git リポジトリでない・git が無い環境では
// 空を返して寛容に振る舞う（履歴なし = 機能が静かに無効になるだけ）。
package history

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Commit は 1 つのコミットの軽量メタ情報。
type Commit struct {
	Hash    string `json:"hash"`
	Short   string `json:"short"`
	Date    string `json:"date"` // author date, RFC3339
	Subject string `json:"subject"`
}

// log のフィールド区切り（subject に出てこない制御文字）。
const sep = "\x1f"

// Log は rel を変更したコミットを新しい順に返す。git repo でなければ (nil, nil)。
func Log(ctx context.Context, root, rel string) ([]Commit, error) {
	if !isRepo(ctx, root) {
		return nil, nil
	}
	out, err := run(ctx, root, "log", "--no-color", "--format=%H"+sep+"%aI"+sep+"%s", "--", rel)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		p := strings.SplitN(line, sep, 3)
		if len(p) < 3 {
			continue
		}
		commits = append(commits, Commit{Hash: p[0], Short: shortHash(p[0]), Date: p[1], Subject: p[2]})
	}
	return commits, nil
}

// Show は rev 時点の rel の内容を返す。rev は呼び出し側で hex 検証済みのこと。
func Show(ctx context.Context, root, rel, rev string) (string, error) {
	return run(ctx, root, "show", "--no-color", rev+":"+rel)
}

func isRepo(ctx context.Context, root string) bool {
	_, err := run(ctx, root, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func run(ctx context.Context, root string, args ...string) (string, error) {
	// core.quotepath=false で非 ASCII パス（日本語ファイル名）の \xxx クォートを抑止。
	full := append([]string{"-C", root, "-c", "core.quotepath=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
