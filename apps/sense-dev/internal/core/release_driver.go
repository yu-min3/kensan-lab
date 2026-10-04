package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ReleasePlan is operator-owned intent, never worker output or authorization.
// Each operation still needs a fresh, independent Gate for its fixed inputs.
type ImageWorkflowPlan struct {
	Ref    string `json:"ref"`
	SHA    string `json:"sha"`
	SHA256 string `json:"sha256"`
}

type ReleasePlan struct {
	ImageWorkflow      *ImageWorkflowPlan `json:"image_workflow,omitempty"`
	SchemaVersion      int                `json:"schema_version"`
	MissionID          string             `json:"mission_id"`
	Repository         string             `json:"repository"`
	TargetEnvironment  string             `json:"target_environment"`
	Impact             string             `json:"impact"`
	Rollback           string             `json:"rollback"`
	PullRequestSummary string             `json:"pull_request_summary"`
}

type ReleaseFlow struct {
	ID             string      `json:"id"`
	AuthorTaskID   string      `json:"author_task_id"`
	AuthorAgentID  string      `json:"author_agent_id"`
	HeadSHA        string      `json:"head_sha"`
	Operation      string      `json:"operation"`
	PolicyVersion  string      `json:"policy_version"`
	PlanSHA256     string      `json:"plan_sha256"`
	ScanRef        ArtifactRef `json:"scan_ref"`
	GateTaskID     string      `json:"gate_task_id"`
	GateAgentID    string      `json:"gate_agent_id"`
	GateGeneration int         `json:"gate_generation"`
	GateExpiresAt  time.Time   `json:"gate_expires_at"`
	DecisionID     string      `json:"decision_id,omitempty"`
	ApprovalID     string      `json:"approval_id,omitempty"`
	IntentID       string      `json:"intent_id,omitempty"`
	Status         string      `json:"status"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func LoadReleasePlan(path string) (ReleasePlan, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Size() > 64<<10 {
		return ReleasePlan{}, errors.New("release plan must be a bounded, operator-owned regular file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ReleasePlan{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var plan ReleasePlan
	if err := decoder.Decode(&plan); err != nil {
		return ReleasePlan{}, errors.New("invalid release plan")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ReleasePlan{}, errors.New("release plan has trailing data")
	}
	if err := plan.validate(); err != nil {
		return ReleasePlan{}, err
	}
	return plan, nil
}

func (p ReleasePlan) validate() error {
	if p.ImageWorkflow != nil && (!workflowRefPattern.MatchString(p.ImageWorkflow.Ref) || !githubCommitPattern.MatchString(p.ImageWorkflow.SHA) || !fullDigest(p.ImageWorkflow.SHA256)) {
		return errors.New("release plan must pin trusted image workflow ref, commit and file hash")
	}
	if p.SchemaVersion != 1 || strings.TrimSpace(p.MissionID) == "" || p.Repository != "yu-min3/kensan-lab" || p.TargetEnvironment != "private-canary" || strings.TrimSpace(p.Impact) == "" || strings.TrimSpace(p.Rollback) == "" || strings.TrimSpace(p.PullRequestSummary) == "" || len(p.Impact) > 3000 || len(p.Rollback) > 4000 || len(p.PullRequestSummary) > 4000 {
		return errors.New("release plan requires a fixed mission, private canary, impact, rollback and PR summary")
	}
	return nil
}

// ReleaseDriver only records Gate work and durable publish intents. Credentials
// and external side effects belong to the separately confined publisher bridge.
type ReleaseDriver struct {
	Store        *Store
	Plan         ReleasePlan
	WorktreeRoot string
	mu           sync.Mutex
}

func releaseFlowID(taskID, head, operation, planHash string) string {
	return digest([]byte(taskID + "\x00" + head + "\x00" + operation + "\x00" + ReleasePolicyVersion + "\x00" + planHash))
}

func (d *ReleaseDriver) Reconcile(ctx context.Context) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Store == nil || !filepath.IsAbs(d.WorktreeRoot) {
		return 0, errors.New("release driver requires Store and absolute worktree root")
	}
	if err := d.Plan.validate(); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	st := d.Store.Snapshot()
	if st.Stopped || st.PausedUntil != nil && time.Now().Before(*st.PausedUntil) {
		return 0, nil
	}
	planBody, _ := json.Marshal(d.Plan)
	planHash := digest(planBody)
	if err := d.Store.update(func(st *State) error {
		for id, f := range st.ReleaseFlows {
			task := st.Tasks[f.AuthorTaskID]
			if f.Status == "sent" || f.Status == "superseded" {
				continue
			}
			intent := st.Intents[f.IntentID]
			inspectOnly := intent.Status == "unknown" || intent.Status == "sending"
			if inspectOnly || f.PlanSHA256 == planHash && f.PolicyVersion == ReleasePolicyVersion && task.HeadSHA == f.HeadSHA {
				continue
			}
			f.Status, f.UpdatedAt = "superseded", time.Now().UTC()
			st.ReleaseFlows[id] = f
			if gate, ok := st.Agents[f.GateAgentID]; ok && gate.Status != "running" {
				gate.Status = "superseded"
				st.Agents[gate.ID] = gate
			}
			if gate, ok := st.Tasks[f.GateTaskID]; ok {
				gate.Status = "superseded"
				st.Tasks[gate.ID] = gate
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	tasks := make([]Task, 0)
	for _, task := range st.Tasks {
		if task.Kind == "change" && task.MissionID == d.Plan.MissionID && task.Status == "publish_wait" {
			tasks = append(tasks, task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	changes := 0
	var failures []error
	for _, task := range tasks {
		ambiguous := false
		for _, prior := range d.Store.Snapshot().ReleaseFlows {
			intent := d.Store.Snapshot().Intents[prior.IntentID]
			if prior.AuthorTaskID == task.ID && (prior.PlanSHA256 != planHash || prior.PolicyVersion != ReleasePolicyVersion || prior.HeadSHA != task.HeadSHA) && (intent.Status == "unknown" || intent.Status == "sending") {
				ambiguous = true
				break
			}
		}
		if ambiguous {
			continue
		}
		proof, err := d.Store.reviewedChangeProof(task.ID)
		if err != nil {
			continue
		} // Simulation and rejected stages are never promoted.
		operations := []string{"branch_push", "pr_create", "merge"}
		if d.Plan.ImageWorkflow != nil && task.Team == App && task.ImageSourceTaskID == "" {
			operations = []string{"branch_push", "image_publish"}
		}
		for _, operation := range operations {
			id := releaseFlowID(task.ID, task.HeadSHA, operation, planHash)
			st = d.Store.Snapshot()
			flow, exists := st.ReleaseFlows[id]
			if !exists {
				if err := d.createFlow(task, proof, operation, planHash); err != nil {
					failures = append(failures, err)
					break
				}
				changes++
				flow = d.Store.Snapshot().ReleaseFlows[id]
			}
			n, err := d.advanceFlow(flow, proof)
			changes += n
			if err != nil {
				failures = append(failures, err)
				break
			}
			flow = d.Store.Snapshot().ReleaseFlows[id]
			if flow.Status != "sent" {
				break
			}
		}
	}
	return changes, errors.Join(failures...)
}

func (d *ReleaseDriver) candidate(task Task, operation string) ReleaseCandidate {
	impact := d.Plan.Impact
	if operation == "pr_create" {
		impact += " Ready PR targeting main; a later separately gated merge triggers private GitOps deployment."
	}
	if operation == "merge" {
		impact += " Merge targets main and triggers the private canary GitOps deployment; CI and current PR head must be checked before merge."
	}
	candidate := ReleaseCandidate{SchemaVersion: 1, Operation: operation, Repository: d.Plan.Repository, Ref: "refs/heads/sense-dev/" + task.ID, HeadSHA: task.HeadSHA, TargetEnvironment: d.Plan.TargetEnvironment, Impact: impact, Rollback: d.Plan.Rollback, PullRequestSummary: d.Plan.PullRequestSummary}
	if operation == "image_publish" && d.Plan.ImageWorkflow != nil {
		planBody, _ := json.Marshal(d.Plan)
		id := releaseFlowID(task.ID, task.HeadSHA, operation, digest(planBody))
		flow := d.Store.Snapshot().ReleaseFlows[id]
		body, _ := d.Store.ReadArtifact(flow.ScanRef.ID)
		var scan ReleaseScan
		_ = json.Unmarshal(body, &scan)
		dispatch := id[:32]
		w := d.Plan.ImageWorkflow
		candidate.ImageRelease = &ImageReleaseSpec{SourceSHA: task.HeadSHA, SourceAppTreeSHA: scan.SourceAppTreeSHA, ImageTag: "sense-" + task.HeadSHA + "-" + dispatch, WorkflowRef: w.Ref, WorkflowSHA: w.SHA, WorkflowPath: CanaryImageWorkflowPath, WorkflowSHA256: w.SHA256, DispatchID: dispatch}
	} else {
		candidate.ImageDeployment = deploymentImageForTask(d.Store.Snapshot(), task)
	}
	return candidate
}

func (d *ReleaseDriver) createFlow(task Task, proof releaseProof, operation, planHash string) error {
	scan, err := ScanGitRangeForTeam(filepath.Join(d.WorktreeRoot, task.ID), task.BaseSHA, task.HeadSHA, operation, "refs/heads/sense-dev/"+task.ID, task.Team)
	if err != nil {
		return err
	}
	if err := d.enrichImageScan(task, &scan); err != nil {
		return err
	}
	scanRef, err := d.Store.recordReleaseScan(scan)
	if err != nil {
		return err
	}
	id := releaseFlowID(task.ID, task.HeadSHA, operation, planHash)
	gateTaskID, err := newID()
	if err != nil {
		return err
	}
	gateAgentID, err := newID()
	if err != nil {
		return err
	}
	return d.Store.update(func(st *State) error {
		if _, exists := st.ReleaseFlows[id]; exists {
			return nil
		}
		if !releaseProofStillCurrent(st, task.ID, proof) || st.Tasks[task.ID].HeadSHA != task.HeadSHA {
			return errors.New("release source changed before flow reservation")
		}
		now := time.Now().UTC()
		flow := ReleaseFlow{ID: id, AuthorTaskID: task.ID, AuthorAgentID: st.Artifacts[proof.Implementation.ID].AgentID, HeadSHA: task.HeadSHA, Operation: operation, PolicyVersion: ReleasePolicyVersion, PlanSHA256: planHash, ScanRef: scanRef, GateTaskID: gateTaskID, GateAgentID: gateAgentID, GateGeneration: 1, GateExpiresAt: now.Add(30 * time.Minute), Status: "binding", UpdatedAt: now}
		if scan.Status == "deny" {
			flow.Status = "denied"
			flow.GateTaskID, flow.GateAgentID = "", ""
		} else {
			reserveReleaseGate(st, task, flow, now)
		}
		st.ReleaseFlows[id] = flow
		st.Events = append(st.Events, event("release_flow_reserved", id, operation))
		return nil
	})
}

func reserveReleaseGate(st *State, source Task, flow ReleaseFlow, now time.Time) {
	st.Tasks[flow.GateTaskID] = Task{ID: flow.GateTaskID, MissionID: source.MissionID, Team: Platform, Kind: "release_gate", Title: "独立 Release Gate: " + flow.Operation, ContractVersion: source.ContractVersion, SourceTaskID: source.ID, BaseSHA: source.BaseSHA, HeadSHA: source.HeadSHA, Status: "binding", CreatedAt: now, UpdatedAt: now}
	st.Agents[flow.GateAgentID] = Agent{ID: flow.GateAgentID, TaskID: flow.GateTaskID, Team: Platform, Role: "release_gate", Provider: "codex", Model: "gpt-6-astra", Status: "binding", SessionGeneration: 1, UpdatedAt: now}
}

func (d *ReleaseDriver) advanceFlow(flow ReleaseFlow, proof releaseProof) (int, error) {
	st := d.Store.Snapshot()
	task := st.Tasks[flow.AuthorTaskID]
	if task.HeadSHA != flow.HeadSHA || flow.PolicyVersion != ReleasePolicyVersion {
		return 0, errors.New("release flow is stale")
	}
	if flow.Status == "denied" || flow.Status == "failed" {
		return 0, nil
	}
	if flow.IntentID != "" {
		intent := st.Intents[flow.IntentID]
		if intent.Status == "sent" && intent.DecisionID == flow.DecisionID && intent.HeadSHA == flow.HeadSHA && intent.Operation == flow.Operation {
			if flow.Status == "sent" {
				return 0, nil
			}
			return 1, d.setFlowStatus(flow, "sent")
		}
		if intent.Status == "sending" || intent.Status == "unknown" {
			return 0, nil
		} // inspect, never blindly resend
	}
	if flow.Status == "sent" {
		return 0, nil
	}
	if flow.Status == "needs_human" {
		approval := st.Approvals[flow.ApprovalID]
		if approval.Status != "approved" || !time.Now().Before(approval.ExpiresAt) {
			return 0, nil
		}
		return 1, d.refreshGate(flow, proof)
	}
	if !time.Now().Before(flow.GateExpiresAt) {
		return 1, d.refreshGate(flow, proof)
	}
	if flow.Status == "binding" {
		gate := st.Agents[flow.GateAgentID]
		if gate.ReviewAuthorID == "" {
			if err := d.Store.BindReleaseGateInputs(gate.ID, flow.AuthorAgentID, []ArtifactRef{proof.Implementation}, flow.ScanRef, d.candidate(task, flow.Operation)); err != nil {
				return 0, err
			}
		}
		err := d.Store.update(func(st *State) error {
			current := st.ReleaseFlows[flow.ID]
			gate := st.Agents[current.GateAgentID]
			if current.GateAgentID != flow.GateAgentID || gate.ReviewAuthorID != flow.AuthorAgentID || len(gate.ReviewInputs) == 0 {
				return errors.New("release Gate binding changed")
			}
			gate.Status, gate.UpdatedAt = "ready", time.Now().UTC()
			st.Agents[gate.ID] = gate
			task := st.Tasks[gate.TaskID]
			task.Status = "ready"
			st.Tasks[task.ID] = task
			current.Status, current.UpdatedAt = "gate_wait", time.Now().UTC()
			st.ReleaseFlows[current.ID] = current
			return nil
		})
		return 1, err
	}
	gate := st.Agents[flow.GateAgentID]
	if gate.Status != "completed" {
		return 0, nil
	}
	if flow.DecisionID == "" {
		var result Attempt
		for _, attempt := range st.Attempts {
			if attempt.AgentID == gate.ID && attempt.Status == "completed" && attempt.OutputRef != nil {
				if result.ID != "" {
					return 0, errors.New("multiple release Gate results")
				}
				result = attempt
			}
		}
		if result.OutputRef == nil {
			return 0, errors.New("release Gate result missing")
		}
		body, err := d.Store.ReadArtifact(result.OutputRef.ID)
		if err != nil {
			return 0, err
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		var output GateOutput
		if err := decoder.Decode(&output); err != nil {
			return 0, errors.New("release Gate output invalid")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return 0, errors.New("release Gate output trailing data")
		}
		candidate := d.candidate(task, flow.Operation)
		decision := ReleaseDecision{ImageRelease: cloneImageRelease(candidate.ImageRelease), ImageDeployment: cloneImageDeployment(candidate.ImageDeployment), AuthorAgentID: flow.AuthorAgentID, GateAgentID: gate.ID, Verdict: output.Verdict, Reason: output.Reason, Operation: flow.Operation, Repository: candidate.Repository, Ref: candidate.Ref, HeadSHA: flow.HeadSHA, TargetEnvironment: candidate.TargetEnvironment, PolicyVersion: flow.PolicyVersion, ArtifactRefs: []ArtifactRef{proof.Implementation}, EvidenceRefs: []ArtifactRef{*result.OutputRef}, ScanRef: flow.ScanRef, SecretFree: output.SecretFree, PrivateTarget: output.PrivateTarget, Reversible: output.Reversible, CIComplete: output.CIComplete, ExpiresAt: flow.GateExpiresAt}
		manifest, err := d.Store.BuildManifest(gate.ID, []string{"isolated-model-worker"})
		if err != nil {
			return 0, err
		}
		candidateRef := gate.ReviewInputs[len(gate.ReviewInputs)-1]
		if _, err := d.Store.matchingGateOutput(decision, manifest, proof, candidateRef); err != nil {
			return 0, err
		}
		for _, ref := range gate.ReviewInputs {
			if st.Artifacts[ref.ID].Kind == "human_approval" {
				var approval ApprovalRequest
				approvalBody, err := d.Store.ReadArtifact(ref.ID)
				if err != nil || json.Unmarshal(approvalBody, &approval) != nil {
					return 0, errors.New("bound approval invalid")
				}
				decision.ApprovalID = approval.ID
			}
		}
		// The flow reserves the decision identity across retries/restarts.
		decision.ID = "release-decision-" + gate.ID
		recorded, exists := d.Store.Snapshot().Decisions[decision.ID]
		if !exists {
			recorded, err = d.Store.RecordReleaseDecision(decision)
			if err != nil {
				return 0, err
			}
		} else if recorded.GateAgentID != gate.ID || recorded.HeadSHA != flow.HeadSHA || recorded.Operation != flow.Operation {
			return 0, errors.New("release decision identity collision")
		}
		flow.DecisionID = recorded.ID
		if recorded.Verdict == "needs_human" {
			approval, err := d.Store.RequestApproval(recorded.ID)
			if err != nil {
				return 0, err
			}
			flow.ApprovalID, flow.Status = approval.ID, "needs_human"
		} else if recorded.Verdict == "deny" {
			flow.Status = "denied"
		} else {
			flow.Status = "publish_wait"
		}
		if err := d.saveFlow(flow); err != nil {
			return 0, err
		}
		return 1, nil
	}
	if flow.Status == "publish_wait" {
		if flow.IntentID != "" {
			return 0, nil
		}
		decision := st.Decisions[flow.DecisionID]
		intent, err := d.Store.PreparePublish(decision.ID, flow.Operation, decision.Repository, decision.Ref, flow.HeadSHA)
		if err != nil {
			return 0, err
		}
		flow.IntentID = intent.ID
		return 1, d.saveFlow(flow)
	}
	return 0, nil
}

func (d *ReleaseDriver) setFlowStatus(flow ReleaseFlow, status string) error {
	flow.Status = status
	return d.saveFlow(flow)
}
func (d *ReleaseDriver) saveFlow(flow ReleaseFlow) error {
	return d.Store.update(func(st *State) error {
		current := st.ReleaseFlows[flow.ID]
		if current.GateAgentID != flow.GateAgentID || st.Tasks[flow.AuthorTaskID].HeadSHA != flow.HeadSHA {
			return errors.New("release flow changed during reconciliation")
		}
		flow.UpdatedAt = time.Now().UTC()
		st.ReleaseFlows[flow.ID] = flow
		return nil
	})
}

func (d *ReleaseDriver) refreshGate(flow ReleaseFlow, proof releaseProof) error {
	// Reuse the approved scan only while it remains fresh. A new scan has a new
	// hash and therefore requires a new exact approval rather than inheriting one.
	scan, err := d.Store.loadReleaseScan(flow.ScanRef, ReleaseDecision{AuthorAgentID: flow.AuthorAgentID, Repository: d.Plan.Repository, Ref: "refs/heads/sense-dev/" + flow.AuthorTaskID, Operation: flow.Operation, HeadSHA: flow.HeadSHA, PolicyVersion: flow.PolicyVersion})
	if err != nil {
		task := d.Store.Snapshot().Tasks[flow.AuthorTaskID]
		scan, err = ScanGitRangeForTeam(filepath.Join(d.WorktreeRoot, task.ID), task.BaseSHA, task.HeadSHA, flow.Operation, "refs/heads/sense-dev/"+task.ID, task.Team)
		if err != nil {
			return err
		}
		if err := d.enrichImageScan(task, &scan); err != nil {
			return err
		}
		flow.ScanRef, err = d.Store.recordReleaseScan(scan)
		if err != nil {
			return err
		}
		flow.ApprovalID = ""
	}
	if scan.Status == "deny" {
		return errors.New("denied release scan cannot be refreshed")
	}
	taskID, err := newID()
	if err != nil {
		return err
	}
	agentID, err := newID()
	if err != nil {
		return err
	}
	return d.Store.update(func(st *State) error {
		current := st.ReleaseFlows[flow.ID]
		if current.GateAgentID != flow.GateAgentID || !releaseProofStillCurrent(st, flow.AuthorTaskID, proof) {
			return errors.New("release flow changed before fresh Gate")
		}
		previous := st.Agents[flow.GateAgentID]
		previous.Status = "superseded"
		st.Agents[previous.ID] = previous
		oldTask := st.Tasks[flow.GateTaskID]
		oldTask.Status = "superseded"
		st.Tasks[oldTask.ID] = oldTask
		flow.GateTaskID, flow.GateAgentID = taskID, agentID
		flow.GateGeneration++
		flow.DecisionID, flow.IntentID, flow.Status = "", "", "binding"
		now := time.Now().UTC()
		flow.GateExpiresAt, flow.UpdatedAt = now.Add(30*time.Minute), now
		reserveReleaseGate(st, st.Tasks[flow.AuthorTaskID], flow, now)
		st.ReleaseFlows[flow.ID] = flow
		st.Events = append(st.Events, event("release_gate_refreshed", flow.ID, fmt.Sprintf("generation %d", flow.GateGeneration)))
		return nil
	})
}

// PublishReady uses the controller's sole Store writer and a separately
// authenticated transport. Unknown results always go through RunPublish inspect.
func (d *ReleaseDriver) PublishReady(ctx context.Context, transport PublishTransport) error {
	if transport == nil {
		return errors.New("publisher transport required")
	}
	body, _ := json.Marshal(d.Plan)
	planHash := digest(body)
	st := d.Store.Snapshot()
	var ids []string
	for id, f := range st.ReleaseFlows {
		task := st.Tasks[f.AuthorTaskID]
		intent := st.Intents[f.IntentID]
		inspectOnly := intent.Status == "unknown" || intent.Status == "sending"
		if inspectOnly || f.PlanSHA256 == planHash && f.PolicyVersion == ReleasePolicyVersion && task.HeadSHA == f.HeadSHA && task.MissionID == d.Plan.MissionID && f.IntentID != "" && f.Status == "publish_wait" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var failures []error
	for _, id := range ids {
		f := d.Store.Snapshot().ReleaseFlows[id]
		if _, err := d.Store.RunPublish(ctx, f.DecisionID, transport); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
