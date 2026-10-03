// Package faults は故障を意図的に起こす Runner を提供する。
//
// mock.Runner は正常系しか通らないので、quota / auth / provider 障害 / timeout を
// 区別できているか、止まった後にどこから再開するかを確かめられない。ここでは
// 故障を台本として与え、controller が待機理由を取り違えないこと、同じ外部操作を
// 二度実行しないことを検証できるようにする。
package faults

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

// Kind は 1 回の実行で起こす故障。空文字は正常終了。
type Kind string

const (
	None     Kind = ""
	Quota    Kind = "quota"    // 枠切れ。待てば直る
	Auth     Kind = "auth"     // 認証切れ。人が要る
	Provider Kind = "provider" // provider 側の一時障害。再試行で直りうる
	Timeout  Kind = "timeout"  // 応答が来ない。成功扱いにしない
)

// Call は 1 回の dispatch の記録。
type Call struct {
	AttemptID  string
	AgentID    string
	Role       string
	Provider   string
	Generation int
	Fault      Kind
}

// Runner は台本どおりに故障を起こす core.Runner。台本を使い切った後は成功する。
type Runner struct {
	mu     sync.Mutex
	script []Kind
	calls  []Call
	// external は「成功した実行＝外部操作をしたとみなすもの」の記録。
	// 重複実行が起きていないかを attempt 単位で確かめるために使う。
	external []string
}

// New は台本を与えて Runner を作る。
func New(script ...Kind) *Runner { return &Runner{script: script} }

// Calls はこれまでの dispatch。
func (r *Runner) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Call(nil), r.calls...)
}

// External は成功した実行の attempt ID。重複が無いことの確認に使う。
func (r *Runner) External() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.external...)
}

func (r *Runner) next() Kind {
	if len(r.script) == 0 {
		return None
	}
	k := r.script[0]
	r.script = r.script[1:]
	return k
}

func (r *Runner) Run(ctx context.Context, d core.Dispatch) (core.RunResult, error) {
	r.mu.Lock()
	fault := r.next()
	r.calls = append(r.calls, Call{
		AttemptID: d.Attempt.ID, AgentID: d.Attempt.AgentID, Role: d.Attempt.Role,
		Provider: d.Attempt.Provider, Generation: d.Attempt.Generation, Fault: fault,
	})
	r.mu.Unlock()

	// session を先に結び付ける。auth / timeout でも「どの session で落ちたか」を
	// 残したいので、故障の前に行う。
	if err := d.BindSession("faults-" + d.Attempt.ID); err != nil {
		return core.RunResult{}, err
	}

	switch fault {
	case Quota:
		return core.RunResult{}, core.RunError{Kind: "quota_wait", Err: errors.New("weekly limit reached")}
	case Auth:
		return core.RunResult{}, core.RunError{Kind: "auth_required", Err: errors.New("provider session is not authenticated")}
	case Provider:
		return core.RunResult{}, core.RunError{Kind: "retry_wait", Err: errors.New("provider returned 503")}
	case Timeout:
		return core.RunResult{}, context.DeadlineExceeded
	}

	r.mu.Lock()
	r.external = append(r.external, d.Attempt.ID)
	r.mu.Unlock()

	b, err := json.Marshal(struct {
		SimulationOnly bool   `json:"simulation_only"`
		AttemptID      string `json:"attempt_id"`
		AgentID        string `json:"agent_id"`
		ManifestHash   string `json:"input_manifest_hash"`
		Result         string `json:"result"`
	}{true, d.Attempt.ID, d.Attempt.AgentID, d.Manifest.InputSHA256, "fault-injection run; no model or code change"})
	return core.RunResult{Output: b}, err
}
