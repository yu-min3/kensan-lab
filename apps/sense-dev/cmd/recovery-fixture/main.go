// recovery-fixture is an operator-invoked, one-turn fixture, never a service.
// Its relay deliberately loses an actual provider result by killing itself.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/isolation"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerclient"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

const mission = "t022-recovery-fixture-v1"

var scope = []string{"isolated-model-worker", "operator-recovery-fixture"}

func main() {
	if filepath.Base(os.Args[0]) == "sense-recovery-relay" {
		if err := relay(); err != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Preflight calls go straight to the unmodified official worker. For a turn,
// relay forwards session+ack and waits for a genuine terminal result. Only
// then does it SIGKILL itself without forwarding that result to the controller.
// No provider failure is converted to success and no model output is fabricated.
func relay() error {
	return relayCommand(exec.Command("/usr/local/bin/sense-dev-worker", os.Args[1:]...), len(os.Args) > 1)
}

func relayCommand(child *exec.Cmd, preflight bool) error {
	child.Stderr = io.Discard
	if preflight {
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		return child.Run()
	}
	input, err := child.StdinPipe()
	if err != nil {
		return err
	}
	output, err := child.StdoutPipe()
	if err != nil {
		return err
	}
	if err = child.Start(); err != nil {
		return err
	}
	defer func() {
		input.Close()
		if child.ProcessState == nil {
			child.Process.Kill()
			child.Wait()
		}
	}()
	decoder := json.NewDecoder(os.Stdin)
	var req workerwire.Request
	if err = decoder.Decode(&req); err != nil {
		return err
	}
	if err = req.Validate(); err != nil {
		return err
	}
	if req.Role != "feedback" || req.Provider != "claude" || req.Model != "claude-opus-5-5" || req.ExistingSession != "" {
		return errors.New("relay accepts only fresh approved feedback")
	}
	if err = json.NewEncoder(input).Encode(req); err != nil {
		return err
	}
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 65536), workerwire.MaxOutput+4096)
	bound := false
	for scanner.Scan() {
		var event workerwire.Event
		if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		if err = event.Validate(); err != nil {
			return err
		}
		if event.Type == "result" {
			if !bound || event.Output == "" {
				return errors.New("cannot inject kill without real bound result")
			}
			if err = child.Wait(); err != nil {
				return err
			}
			if err := syscall.Kill(os.Getpid(), syscall.SIGKILL); err != nil {
				return err
			}
			select {}
		}
		if err = json.NewEncoder(os.Stdout).Encode(event); err != nil {
			return err
		}
		if event.Type == "session" {
			if bound {
				return errors.New("duplicate session")
			}
			var ack workerwire.Ack
			if err = decoder.Decode(&ack); err != nil {
				return err
			}
			if err = ack.Validate(event.SessionID); err != nil {
				return err
			}
			if err = json.NewEncoder(input).Encode(ack); err != nil {
				return err
			}
			bound = true
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	return child.Wait()
}

func run() error {
	phase := flag.String("phase", "", "init, kill-turn, resume, recovery-turn, audit")
	state := flag.String("state", "", "dedicated fixture state root")
	source := flag.String("source", "", "immutable source checkout")
	work := flag.String("worktrees", "", "dedicated fixture task root")
	runtime := flag.String("runtime", "", "fixed fixture runtime root")
	auth := flag.String("auth", "", "dedicated Claude subscription auth")
	bwrap := flag.String("bwrap", "", "fixed bubblewrap launcher")
	base := flag.String("base", "", "full immutable source SHA")
	flag.Parse()
	if *phase == "" || !filepath.IsAbs(*state) {
		return errors.New("phase and absolute state required")
	}
	if *phase == "init" {
		if _, err := os.Lstat(filepath.Join(*state, "state.json")); !os.IsNotExist(err) {
			return errors.New("init requires fresh state")
		}
	}
	s, err := core.Open(*state)
	if err != nil {
		return err
	}
	defer s.Close()
	if *phase == "init" {
		if len(s.Snapshot().Tasks) != 0 {
			return errors.New("fixture not empty")
		}
		if len(*base) != 40 {
			return errors.New("fixed source SHA required")
		}
		if err = s.SeedKnowledge(); err != nil {
			return err
		}
		for _, team := range []core.Team{core.Platform, core.App} {
			task, agents, err := s.CreatePlannedTask(mission, team, "analysis", "Reply only with JSON containing your team and the exact private recovery sentinel from your own memo. Do not use tools or include another team. This is a context audit, not source development.", "recovery-v1")
			if err != nil {
				return err
			}
			if err = s.SetBaseSHA(task.ID, *base); err != nil {
				return err
			}
			memo := "Private recovery sentinel: " + sentinel(team) + ". Reply ONLY with JSON containing your team and this sentinel. Do not use tools. Do not include any other team's context."
			if _, err = s.PutArtifact(agents[0].ID, "memo", []byte(memo)); err != nil {
				return err
			}
		}
		return summary(s)
	}
	st := s.Snapshot()
	if len(st.Tasks) != 2 || len(st.Agents) != 2 {
		return errors.New("unexpected fixture population")
	}
	for _, t := range st.Tasks {
		if t.MissionID != mission || t.Kind != "analysis" || t.ContractVersion != "recovery-v1" {
			return errors.New("foreign task")
		}
	}
	for _, a := range st.Agents {
		if a.Role != "feedback" || a.Provider != "claude" || a.Model != "claude-opus-5-5" {
			return errors.New("foreign agent")
		}
	}
	switch *phase {
	case "resume":
		if len(st.Attempts) != 2 {
			return errors.New("two original attempts required")
		}
		for _, a := range st.Agents {
			if a.Status != "failed" || a.SessionGeneration != 1 || a.SessionID == "" {
				return errors.New("both old sessions must be failed and bound")
			}
		}
		for id := range st.Agents {
			if err = s.ResumeAgent(id, "Operator verified fixture-only session history archival; reconstruct generation from private memo and team artifacts"); err != nil {
				return err
			}
		}
	case "kill-turn", "recovery-turn":
		generation := 1
		wantStatus := "failed"
		program := "/usr/local/bin/sense-recovery-relay"
		lo, hi := 0, 1
		if *phase == "recovery-turn" {
			generation = 2
			wantStatus = "completed"
			program = "/usr/local/bin/sense-dev-worker"
			lo, hi = 2, 3
		}
		if len(st.Attempts) < lo || len(st.Attempts) > hi {
			return errors.New("phase turn budget exhausted")
		}
		for _, a := range st.Agents {
			if a.SessionGeneration != generation || (a.Status != "ready" && a.Status != wantStatus) {
				return errors.New("unexpected phase state")
			}
		}
		config := isolation.Config{Bubblewrap: *bwrap, RuntimeRoot: *runtime, AuthHome: *auth, ControllerState: *state, Worktree: *source, ReadOnlyWorktree: true}
		runner := workerclient.Runner{Claude: config, Codex: config, WorkerProgram: program, Timeout: 120 * time.Second, Worktrees: worktree.Manager{Source: *source, Root: *work}}
		worked, err := s.Tick(context.Background(), runner, scope)
		if err != nil {
			return err
		}
		if !worked {
			return errors.New("no ready fixture agent")
		}
		after := s.Snapshot()
		if len(after.Attempts) != len(st.Attempts)+1 {
			return errors.New("turn count mismatch")
		}
		for id, a := range after.Attempts {
			if _, old := st.Attempts[id]; !old && (a.Status != wantStatus || a.SessionID == "" || a.Generation != generation) {
				return errors.New("unexpected turn outcome; stop without retry")
			}
		}
	case "audit":
		if err = audit(s); err != nil {
			return err
		}
	default:
		return errors.New("unknown fixture phase")
	}
	return summary(s)
}
func sentinel(team core.Team) string {
	return "T022_PRIVATE_" + strings.ToUpper(string(team)) + "_9B6E"
}
func summary(s *core.Store) error {
	st := s.Snapshot()
	return json.NewEncoder(os.Stdout).Encode(struct {
		Tasks    map[string]core.Task    `json:"tasks"`
		Agents   map[string]core.Agent   `json:"agents"`
		Attempts map[string]core.Attempt `json:"attempts"`
	}{st.Tasks, st.Agents, st.Attempts})
}
func audit(s *core.Store) error {
	st := s.Snapshot()
	if len(st.Attempts) != 4 {
		return errors.New("four attempts required")
	}
	sessions := map[string]bool{}
	for _, a := range st.Agents {
		if a.Status != "completed" || a.SessionGeneration != 2 {
			return errors.New("recovery incomplete")
		}
		for generation := 1; generation <= 2; generation++ {
			found := false
			for _, turn := range st.Attempts {
				if turn.AgentID == a.ID && turn.Generation == generation {
					if found || turn.SessionID == "" || sessions[turn.SessionID] {
						return errors.New("duplicate/missing session")
					}
					found = true
					sessions[turn.SessionID] = true
					expected := "failed"
					if generation == 2 {
						expected = "completed"
					}
					if turn.Status != expected {
						return errors.New("attempt history changed")
					}
				}
			}
			if !found {
				return errors.New("generation missing")
			}
		}
		m, err := s.BuildManifest(a.ID, scope)
		if err != nil {
			return err
		}
		prompt, err := s.ManifestPrompt(m)
		if err != nil {
			return err
		}
		other := core.App
		if a.Team == core.App {
			other = core.Platform
		}
		if !strings.Contains(prompt, sentinel(a.Team)) || strings.Contains(prompt, sentinel(other)) {
			return errors.New("private context mixed or lost")
		}
		for _, turn := range st.Attempts {
			if turn.AgentID == a.ID && turn.Generation == 2 {
				if turn.InputHash != m.InputSHA256 || turn.OutputRef == nil {
					return errors.New("recovered manifest not pinned")
				}
				output, err := s.ReadArtifact(turn.OutputRef.ID)
				if err != nil {
					return err
				}
				if !strings.Contains(string(output), sentinel(a.Team)) || strings.Contains(string(output), sentinel(other)) {
					return errors.New("model recovery output mixed or lost")
				}
			}
		}
	}
	return nil
}
