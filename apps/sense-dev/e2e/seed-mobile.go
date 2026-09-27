// Command seed-mobile creates a simulation-only App/Platform exchange in a
// temporary state directory for the private mobile browser test. It is not a
// production setup command and does not invoke models or publish anything.
package main

import (
	"flag"
	"fmt"
	"log"
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
		if err := store.SetAgentSession(agent.ID, agent.Provider, agent.Model, "fixture-session-"+agent.ID, manifest.InputSHA256, 1); err != nil {
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
	for _, width := range []int{360, 390, 430} {
		decision, err := store.RecordReleaseDecision(core.ReleaseDecision{
			AuthorAgentID:     platformAgent.ID,
			GateAgentID:       gate.ID,
			Verdict:           "needs_human",
			Reason:            fmt.Sprintf("画面幅 %d の判断（模擬）", width),
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
}
