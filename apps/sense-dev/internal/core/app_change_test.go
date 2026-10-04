package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readyAppChange(t *testing.T, runner releaseReadyRunner) (*Store, Task, []Agent) {
	t.Helper()
	s := testStore(t)
	task, stages, err := s.CreatePlannedTask("app-mission", App, "change", "canary feature", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBaseSHA(task.ID, runner.base); err != nil {
		t.Fatal(err)
	}
	for range stages {
		if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("App stage: worked=%t err=%v", worked, err)
		}
	}
	return s, s.Snapshot().Tasks[task.ID], stages
}

func TestAppChangeIndependentPipelineAndDeploymentWait(t *testing.T) {
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	s, task, stages := readyAppChange(t, releaseReadyRunner{base: base, head: head})
	if task.Status != "publish_wait" {
		t.Fatalf("App candidate status=%s", task.Status)
	}
	proof, err := s.reviewedChangeProof(task.ID)
	if err != nil {
		t.Fatalf("App candidate proof: %v", err)
	}
	sessions := map[string]bool{}
	for _, stage := range stages {
		agent := s.Snapshot().Agents[stage.ID]
		if agent.Team != App || agent.Status != "completed" {
			t.Fatalf("invalid App stage: %+v", agent)
		}
		if agent.Role == "verification" {
			if agent.SessionID != "" {
				t.Fatal("verifier has model session")
			}
			continue
		}
		if sessions[agent.SessionID] || agent.SessionID == "" {
			t.Fatal("App stages shared a session")
		}
		sessions[agent.SessionID] = true
	}
	acceptance, recipient, err := s.CreateLinkedAcceptanceTask(task.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
		t.Fatalf("undeployed handoff=%d %v", n, err)
	}
	message := Message{CorrelationID: task.MissionID, FromAgent: stages[4].ID, ToAgent: recipient.ID, SourceTask: task.ID, TargetTask: acceptance.ID, Kind: "change_ready", ContractVersion: task.ContractVersion, HeadSHA: head, ArtifactRefs: []ArtifactRef{proof.Implementation, proof.Verification, proof.QualityReview}}
	if _, err := s.SendMessage(message); err == nil {
		t.Fatal("manual handoff bypassed deployment wait")
	}
	if _, ok, err := s.ClaimNext(time.Now()); err != nil || ok {
		t.Fatalf("undeployed acceptance claimed: %t %v", ok, err)
	}
	seedAppDeployment(t, s, task)
	sent, err := s.SendMessage(message)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReceiveMessage(recipient.ID, sent.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBaseSHA(acceptance.ID, head); err != nil {
		t.Fatal(err)
	}
	manifest, err := s.BuildManifest(recipient.ID, []string{"isolated-model-worker"})
	if err != nil || len(manifest.Inbox) != 3 || manifest.DeploymentEvidence == nil {
		t.Fatalf("App handoff manifest: %+v %v", manifest, err)
	}
	prompt, err := s.ManifestPrompt(manifest)
	if err != nil || strings.Contains(prompt, "Platform team:") || !strings.Contains(prompt, "App team: lead application development") || !strings.Contains(prompt, "Evaluate the deployed user path") {
		t.Fatalf("App private context: %v", err)
	}
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
		t.Fatalf("duplicate handoff=%d %v", n, err)
	}
	if worked, err := s.Tick(context.Background(), acceptanceRunner{base: base, head: head, verdict: "pass", observed: "deployed canary feature returns expected response"}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("App acceptance: %t %v", worked, err)
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 1 {
		t.Fatalf("App outcome=%d %v", n, err)
	}
	if s.Snapshot().Tasks[task.ID].Status != "done" {
		t.Fatal("deployed accepted App change did not complete")
	}
	if s.Snapshot().Tasks[acceptance.ID].Status != "done" {
		t.Fatal("App acceptance not completed")
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 0 {
		t.Fatalf("duplicate App outcome=%d %v", n, err)
	}
}

func TestAppChangeRejectedVerificationOrReviewCannotHandoff(t *testing.T) {
	for _, tc := range []struct {
		name         string
		verification bool
		verdict      string
		wrongHash    bool
	}{
		{name: "verification", verification: true}, {name: "review", verdict: "fail"}, {name: "review-hash", wrongHash: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			task, stages, err := s.CreatePlannedTask("app-mission", App, "change", "canary feature", "v1")
			if err != nil {
				t.Fatal(err)
			}
			base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
			if err := s.SetBaseSHA(task.ID, base); err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.CreateLinkedAcceptanceTask(task.ID, "consumer scenario"); err != nil {
				t.Fatal(err)
			}
			limit := len(stages)
			if tc.verification {
				limit = 4
			}
			for range limit {
				if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: head, failVerification: tc.verification, reviewVerdict: tc.verdict, wrongReviewHash: tc.wrongHash}, []string{"isolated-model-worker"}); err != nil || !worked {
					t.Fatalf("stage: %t %v", worked, err)
				}
			}
			if _, err := s.reviewedChangeProof(task.ID); err == nil {
				t.Fatal("rejected App candidate has release proof")
			}
			if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
				t.Fatalf("rejected App handoff=%d %v", n, err)
			}
		})
	}
}

func TestAppAcceptanceFailureCorrectsOwnChangeAndRetestsNewDeployment(t *testing.T) {
	base, head, corrected := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	s, task, stages := readyAppChange(t, releaseReadyRunner{base: base, head: head})
	acceptance, recipient, err := s.CreateLinkedAcceptanceTask(task.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	seedAppDeployment(t, s, task)
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("initial handoff=%d %v", n, err)
	}
	if err := s.SetBaseSHA(acceptance.ID, head); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.Tick(context.Background(), acceptanceRunner{base: base, head: head, verdict: "fail", observed: "feature returns 500"}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("acceptance: %t %v", worked, err)
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 1 {
		t.Fatalf("failed outcome=%d %v", n, err)
	}
	state := s.Snapshot()
	if state.Tasks[task.ID].Status != "ready" || state.Tasks[acceptance.ID].Status != "revision_wait" || state.Agents[stages[2].ID].SessionGeneration != 2 {
		t.Fatal("failed App change did not reopen correction")
	}
	correction, err := s.BuildManifest(stages[2].ID, []string{"isolated-model-worker"})
	if err != nil || len(correction.Inbox) != 1 {
		t.Fatalf("App correction missing pinned failure: %+v %v", correction, err)
	}
	for range 3 {
		if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: corrected}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("correction: %t %v", worked, err)
		}
	}
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
		t.Fatalf("old deployment authorized new revision=%d %v", n, err)
	}
	seedAppDeployment(t, s, s.Snapshot().Tasks[task.ID])
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("retest handoff=%d %v", n, err)
	}
	if _, ok, err := s.ClaimNext(time.Now()); err != nil || ok {
		t.Fatalf("acceptance ran before checkout advance: %t %v", ok, err)
	}
	if err := s.AdvanceAcceptanceBase(acceptance.ID, head, corrected); err != nil {
		t.Fatal(err)
	}
	manifest, err := s.BuildManifest(recipient.ID, []string{"isolated-model-worker"})
	if err != nil || manifest.PreviousAcceptance == nil || manifest.Generation != 2 {
		t.Fatalf("retest lost prior scenario: %+v %v", manifest, err)
	}
	if worked, err := s.Tick(context.Background(), acceptanceRunner{base: base, head: corrected, verdict: "pass", observed: "same deployed operation returns 200"}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("retest: %t %v", worked, err)
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 1 {
		t.Fatalf("retest outcome=%d %v", n, err)
	}
	if s.Snapshot().Tasks[acceptance.ID].Status != "done" {
		t.Fatal("corrected App scenario not accepted")
	}
}

func TestAppSameTeamMessagesRemainLimitedToLinkedAcceptance(t *testing.T) {
	s := testStore(t)
	first, err := s.CreateTask("mission", App, "analysis", "one", "v1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateTask("mission", App, "analysis", "two", "v1")
	if err != nil {
		t.Fatal(err)
	}
	from, err := s.AddAgent(first.ID, "feedback", "claude", "opus")
	if err != nil {
		t.Fatal(err)
	}
	to, err := s.AddAgent(second.ID, "feedback", "claude", "opus")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := s.PutArtifact(from.ID, "feedback", []byte("private memo"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendMessage(Message{CorrelationID: "mission", FromAgent: from.ID, ToAgent: to.ID, SourceTask: first.ID, TargetTask: second.ID, Kind: "change_ready", ContractVersion: "v1", ArtifactRefs: []ArtifactRef{artifactRef(artifact)}}); err == nil {
		t.Fatal("unlinked same-team exchange accepted")
	}
}

func TestTeamKnowledgeUpgradesOriginalDefaultsAndPreservesCustom(t *testing.T) {
	s := testStore(t)
	legacy := "Current mission: test the Golden Path as a consumer. Preserve web-stateless, web-stateful, and batch differences. Feedback returns to Platform as a versioned artifact."
	old, err := s.PutArtifact("system", "knowledge-app", []byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	custom, err := s.PutArtifact("system", "knowledge-platform", []byte("operator approved custom platform scope"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	current, _ := latest(s.Snapshot(), "system", "knowledge-app")
	if current.Version != old.Version+1 {
		t.Fatal("legacy knowledge not upgraded")
	}
	body, err := s.ReadArtifact(current.ID)
	if err != nil || !strings.Contains(string(body), "canary application") {
		t.Fatalf("App knowledge=%s %v", body, err)
	}
	oldBody, err := s.ReadArtifact(old.ID)
	if err != nil || string(oldBody) != legacy {
		t.Fatal("old artifact changed")
	}
	platform, _ := latest(s.Snapshot(), "system", "knowledge-platform")
	if platform.ID != custom.ID {
		t.Fatal("custom knowledge overwritten")
	}
	count := len(s.Snapshot().Artifacts)
	if err := s.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Artifacts) != count {
		t.Fatal("seed duplicated current knowledge")
	}
}

// The host receipt contract is tested separately in deployment_test.go. These
// fixtures represent a trusted GitOps observation, never a model output.
func seedAppDeployment(t *testing.T, s *Store, task Task) {
	t.Helper()
	decisionID, intentID := "deploy-decision-"+task.HeadSHA, "deploy-intent-"+task.HeadSHA
	observation := DeploymentReceipt{ObservedRelease: "v2", TaskID: task.ID, DecisionID: decisionID, IntentID: intentID, HeadSHA: task.HeadSHA, Revision: strings.Repeat("e", 40), ImageSourceSHA: strings.Repeat("e", 40), ImageDigest: "sha256:" + strings.Repeat("d", 64), Environment: "private-canary", Status: "healthy", UserPath: "private user route"}
	body, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := s.PutArtifact("system", "deployment_observation", body)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.update(func(st *State) error {
		var author Agent
		for _, agent := range st.Agents {
			if agent.TaskID == task.ID && agent.Role == "implementation" {
				author = agent
			}
		}
		now := time.Now().UTC()
		st.Decisions[decisionID] = ReleaseDecision{ID: decisionID, AuthorAgentID: author.ID, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/fixture", Verdict: "allow", Operation: "deploy", HeadSHA: task.HeadSHA, TargetEnvironment: "private-canary", PolicyVersion: ReleasePolicyVersion, ExpiresAt: now.Add(time.Hour)}
		st.Intents[intentID] = PublishIntent{ID: intentID, DecisionID: decisionID, Operation: "deploy", HeadSHA: task.HeadSHA, ExternalID: strings.Repeat("e", 40), Status: "sent", AuthorizedAt: now, ExpiresAt: now.Add(time.Minute), Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/fixture", TargetEnvironment: "private-canary", PolicyVersion: ReleasePolicyVersion}
		st.Deployments[task.ID] = DeploymentReceipt{ObservedRelease: "v2", TaskID: task.ID, DecisionID: decisionID, IntentID: intentID, HeadSHA: task.HeadSHA, Revision: strings.Repeat("e", 40), ImageSourceSHA: strings.Repeat("e", 40), ImageDigest: "sha256:" + strings.Repeat("d", 64), Environment: "private-canary", Status: "healthy", UserPath: "private user route", EvidenceRef: artifactRef(artifact), RecordedAt: now}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAppDeploymentEvidenceCorruptionRejectsManifestAndPrompt(t *testing.T) {
	s, task, _ := readyAppChange(t, releaseReadyRunner{base: strings.Repeat("a", 40), head: strings.Repeat("b", 40)})
	_, recipient, err := s.CreateLinkedAcceptanceTask(task.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	seedAppDeployment(t, s, task)
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("handoff=%d %v", n, err)
	}
	manifest, err := s.BuildManifest(recipient.ID, nil)
	if err != nil || manifest.DeploymentEvidence == nil {
		t.Fatalf("manifest: %+v %v", manifest, err)
	}
	artifact := s.Snapshot().Artifacts[manifest.DeploymentEvidence.ID]
	if err := os.WriteFile(filepath.Join(s.root, artifact.Path), []byte("tampered deployment user path"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BuildManifest(recipient.ID, nil); err == nil {
		t.Fatal("corrupt deployment evidence entered manifest")
	}
	if _, err := s.ManifestPrompt(manifest); err == nil {
		t.Fatal("corrupt deployment evidence entered prompt")
	}
}

func TestAppAcceptanceDoesNotStayDoneAfterDeploymentInvalidation(t *testing.T) {
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	s, task, _ := readyAppChange(t, releaseReadyRunner{base: base, head: head})
	acceptance, _, err := s.CreateLinkedAcceptanceTask(task.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	seedAppDeployment(t, s, task)
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("handoff=%d %v", n, err)
	}
	if worked, err := s.Tick(context.Background(), acceptanceRunner{base: base, head: head, verdict: "pass", observed: "user operation returns 200"}, nil); err != nil || !worked {
		t.Fatalf("acceptance: %t %v", worked, err)
	}
	if err := s.update(func(st *State) error {
		receipt := st.Deployments[task.ID]
		receipt.Status = "unhealthy"
		st.Deployments[task.ID] = receipt
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 0 {
		t.Fatalf("invalid deployment outcome=%d %v", n, err)
	}
	if status := s.Snapshot().Tasks[acceptance.ID].Status; status != "decision_wait" {
		t.Fatalf("invalid deployment retained acceptance status=%s", status)
	}
}
