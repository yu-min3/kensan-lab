// Command seed-mobile creates a simulation-only App/Platform exchange in a
// temporary state directory for the private mobile browser test. It is not a
// production setup command and does not invoke models or publish anything.
package main

import (
	"flag"
	"log"

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
}
