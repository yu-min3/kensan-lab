// Command seed-mobile creates a simulation-only App/Platform exchange in a
// temporary state directory for the private mobile browser test. It is not a
// production setup command and does not invoke models or publish anything.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func main() {
	data := flag.String("data", "", "absolute temporary state directory")
	flag.Parse()
	if *data == "" {
		log.Fatal("-data is required")
	}
	store, err := core.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		log.Fatal(err)
	}
	platform, err := store.CreateTask("mobile-fixture", core.Platform, "change", "Golden Path 契約の修正", "fixture-v1")
	if err != nil {
		log.Fatal(err)
	}
	app, err := store.CreateTask("mobile-fixture", core.App, "acceptance", "既存 App の受入再試験", "fixture-v1")
	if err != nil {
		log.Fatal(err)
	}
	platformAgent, err := store.AddAgent(platform.ID, "implementation_review", "codex", "gpt-6-astra")
	if err != nil {
		log.Fatal(err)
	}
	appAgent, err := store.AddAgent(app.ID, "app_acceptance", "claude", "opus")
	if err != nil {
		log.Fatal(err)
	}
	type exchange struct {
		from, to                                           core.Agent
		kind, artifact, body, scenario, expected, observed string
	}
	exchanges := []exchange{
		{platformAgent, appAgent, "change_ready", "reviewed_contract", "fixture initial contract", "", "", ""},
		{appAgent, platformAgent, "acceptance_failed", "acceptance_feedback", "fixture failed acceptance", "existing app login", "SSO session retained", "session lost"},
		{platformAgent, appAgent, "change_ready", "corrected_contract", "fixture corrected contract", "", "", ""},
		{appAgent, platformAgent, "acceptance_passed", "acceptance_result", "fixture passed acceptance", "existing app login", "SSO session retained", "SSO session retained"},
	}
	var prior string
	for i, item := range exchanges {
		artifact, err := store.PutArtifact(item.from.ID, item.artifact, []byte(item.body))
		if err != nil {
			log.Fatal(err)
		}
		message := core.Message{CorrelationID: "mobile-fixture", FromAgent: item.from.ID, ToAgent: item.to.ID, SourceTask: item.from.TaskID, TargetTask: item.to.TaskID, Kind: item.kind, ArtifactRefs: []core.ArtifactRef{{ID: artifact.ID, Version: artifact.Version, SHA256: artifact.SHA256}}, ContractVersion: "fixture-v1", ScenarioID: item.scenario, Expected: item.expected, Observed: item.observed}
		if i > 0 {
			message.ReplyTo = prior
		}
		created, err := store.SendMessage(message)
		if err != nil {
			log.Fatal(err)
		}
		if err := store.ReceiveMessage(item.to.ID, created.ID); err != nil {
			log.Fatal(err)
		}
		prior = created.ID
	}
	for _, width := range []int{360, 390, 430} {
		if _, err := store.AskQuestion(appAgent.ID, fmt.Sprintf("画面幅 %d の契約確認", width)); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := store.AskQuestion(appAgent.ID, "一巡確認の契約質問"); err != nil {
		log.Fatal(err)
	}
	sha := strings.Repeat("a", 40)
	if err := store.SetHeadSHA(platform.ID, sha); err != nil {
		log.Fatal(err)
	}
	gate, err := store.AddAgent(platform.ID, "release_gate", "codex", "gpt-6-astra")
	if err != nil {
		log.Fatal(err)
	}
	for _, agent := range []core.Agent{platformAgent, gate} {
		manifest, err := store.BuildManifest(agent.ID, nil)
		if err != nil {
			log.Fatal(err)
		}
		if err := store.SetAgentSession(agent.ID, agent.Provider, agent.Model, "mock-fixture-session-"+agent.ID, manifest.InputSHA256, 1); err != nil {
			log.Fatal(err)
		}
	}
	source, err := store.PutArtifact(platformAgent.ID, "fixture_decision_source", []byte("fixture proposal; never publish"))
	if err != nil {
		log.Fatal(err)
	}
	evidence, err := store.PutArtifact(gate.ID, "fixture_gate_evidence", []byte("fixture needs human; never publish"))
	if err != nil {
		log.Fatal(err)
	}
	for _, label := range []string{"画面幅 360", "画面幅 390", "画面幅 430", "一巡確認", "一巡差し戻し"} {
		decision, err := store.RecordReleaseDecision(core.ReleaseDecision{
			AuthorAgentID:     platformAgent.ID,
			GateAgentID:       gate.ID,
			Verdict:           "needs_human",
			Reason:            label + " の判断（模擬）",
			Operation:         "merge",
			Repository:        "yu-min3/kensan-lab",
			Ref:               "refs/heads/fixture/mobile",
			HeadSHA:           sha,
			TargetEnvironment: "private-canary",
			PolicyVersion:     core.ReleasePolicyVersion,
			ArtifactRefs:      []core.ArtifactRef{{ID: source.ID, Version: source.Version, SHA256: source.SHA256}},
			EvidenceRefs:      []core.ArtifactRef{{ID: evidence.ID, Version: evidence.Version, SHA256: evidence.SHA256}},
			ExpiresAt:         time.Now().Add(time.Hour),
		})
		if err != nil {
			log.Fatal(err)
		}
		if _, err := store.RequestApproval(decision.ID); err != nil {
			log.Fatal(err)
		}
	}
	// A simulated unknown outbox result exercises the UI without sending anything.
	reportAt := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC) // JST 20:00
	report, created, err := store.QueueDailyReport(reportAt)
	if err != nil || !created {
		log.Fatalf("queue fixture report: created=%t err=%v", created, err)
	}
	if err := store.ConfigureReportDestination(report.Date, "fixture-inbox"); err != nil {
		log.Fatal(err)
	}
	sending, err := store.BeginReportDelivery(report.Date, reportAt)
	if err != nil {
		log.Fatal(err)
	}
	if err := store.FinishReportDelivery(report.Date, sending.AttemptID, "unknown", "", reportAt); err != nil {
		log.Fatal(err)
	}
	// Display fixtures use real envelope validation but no model sessions.
	development, developers, err := store.CreatePlannedTask("mobile-fixture", core.App, "change", "canary の機能開発（模擬）", "fixture-v1")
	if err != nil {
		log.Fatal(err)
	}
	type fixtureFeedback struct {
		message         core.Message
		analysis        core.Task
		verdict, status string
	}
	var feedbackFixtures []fixtureFeedback
	for _, spec := range []struct{ summary, verdict, status string }{
		{"配備ログの不足（模擬）", "", "pending_decision"},
		{"template の改善（模擬）", "adopt", "improving"},
		{"配備後の同じ操作（模擬）", "adopt", "retest_ready"},
		{"公開設定の改善（模擬）", "adopt", "needs_human"},
	} {
		analysis, agents, err := store.CreatePlannedTask("mobile-fixture", core.Platform, "analysis", spec.summary+"の判定", "fixture-v1")
		if err != nil {
			log.Fatal(err)
		}
		artifact, err := store.PutPlatformFeedback(developers[2].ID, core.PlatformFeedback{SchemaVersion: 1, Category: "operability", Summary: spec.summary, Expected: "同じ操作を観測できる", Observed: "ログで原因が分からない", Reproduce: "canary の利用者操作を再実行する"})
		if err != nil {
			log.Fatal(err)
		}
		message, err := store.SendMessage(core.Message{CorrelationID: "mobile-fixture", FromAgent: developers[2].ID, ToAgent: agents[0].ID, SourceTask: development.ID, TargetTask: analysis.ID, Kind: "platform_feedback", ContractVersion: "fixture-v1", ArtifactRefs: []core.ArtifactRef{{ID: artifact.ID, Version: artifact.Version, SHA256: artifact.SHA256}}})
		if err != nil {
			log.Fatal(err)
		}
		if err := store.ReceiveMessage(agents[0].ID, message.ID); err != nil {
			log.Fatal(err)
		}
		var decisionID string
		if spec.verdict != "" {
			decisionArtifact, err := store.PutPlatformDecision(agents[0].ID, core.PlatformDecision{SchemaVersion: 1, FeedbackMessageID: message.ID, Verdict: spec.verdict, Reason: "採用理由: 利用者操作の失敗原因を追えるようにする（模擬）"})
			if err != nil {
				log.Fatal(err)
			}
			decision, err := store.SendMessage(core.Message{CorrelationID: "mobile-fixture", FromAgent: agents[0].ID, ToAgent: developers[2].ID, SourceTask: analysis.ID, TargetTask: development.ID, Kind: "platform_decision", ContractVersion: "fixture-v1", ReplyTo: message.ID, ArtifactRefs: []core.ArtifactRef{{ID: decisionArtifact.ID, Version: decisionArtifact.Version, SHA256: decisionArtifact.SHA256}}})
			if err != nil {
				log.Fatal(err)
			}
			if err := store.ReceiveMessage(developers[2].ID, decision.ID); err != nil {
				log.Fatal(err)
			}
			decisionID = decision.ID
		}
		feedbackFixtures = append(feedbackFixtures, fixtureFeedback{message: message, analysis: analysis, verdict: decisionID, status: spec.status})
	}
	// Freeze presentation phases only in this simulation state; no receipt or
	// approval fixture may be used as production release evidence.
	fixture := store.Snapshot()
	for _, agent := range developers {
		a := fixture.Agents[agent.ID]
		a.Status = "auth_required"
		fixture.Agents[a.ID] = a
	}
	for _, item := range feedbackFixtures {
		loop := core.FeedbackLoop{FeedbackID: item.message.ID, AnalysisTaskID: item.analysis.ID, DecisionID: item.verdict, Status: item.status}
		if item.status == "needs_human" {
			loop.HumanCategories = []string{"publication", "auth"}
			loop.HumanReasons = []string{"公開 host と認証の変更を含む改善提案（模擬）"}
		}
		for id, a := range fixture.Agents {
			if a.TaskID == item.analysis.ID {
				a.Status = "auth_required"
				fixture.Agents[id] = a
			}
		}
		if item.status != "pending_decision" {
			improvementID := "fixture-improvement-" + item.message.ID
			improvement := core.Task{ID: improvementID, MissionID: "mobile-fixture", Team: core.Platform, Kind: "change", Title: "Platform 改善（模擬）", Status: "ready", ContractVersion: "fixture-v1", FeedbackMessageID: item.message.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
			fixture.Tasks[improvementID] = improvement
			loop.ImprovementTaskID = improvementID
			if item.status == "retest_ready" {
				retestID := "fixture-retest-" + item.message.ID
				fixture.Tasks[retestID] = core.Task{ID: retestID, MissionID: "mobile-fixture", Team: core.App, Kind: "acceptance", Title: "App の同じ操作を再確認（模擬）", Status: "ready", ContractVersion: "fixture-v1", SourceTaskID: improvementID, FeedbackMessageID: item.message.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()}
				loop.RetestTaskID = retestID
			}
		}
		fixture.FeedbackLoops[loop.FeedbackID] = loop
	}
	for id, approval := range fixture.Approvals {
		approval.HumanCategories = []string{"auth", "secret", "publication", "control", "data_destruction"}
		approval.HumanReasons = []string{"公開 host・認証設定の変更を含むため Yu の判断が必要（模擬）"}
		approval.PolicyVersion = core.ReleasePolicyVersion
		decision := fixture.Decisions[approval.DecisionID]
		approval.AuthorAgentID = decision.AuthorAgentID
		decision.HumanCategories, decision.HumanReasons = approval.HumanCategories, approval.HumanReasons
		fixture.Decisions[decision.ID] = decision
		fixture.Approvals[id] = approval
	}
	if err := store.Close(); err != nil {
		log.Fatal(err)
	}
	body, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*data, "state.json"), body, 0600); err != nil {
		log.Fatal(err)
	}

}
