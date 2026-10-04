package core

import (
	"strings"
	"testing"
	"time"
)

func imageCorrectionState() (State, Task, Task) {
	st := NewState()
	now := time.Now().UTC()
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	root := Task{ID: "root", MissionID: "mission", Team: App, Kind: "change", Title: "feature scenario", ContractVersion: "v1", HeadSHA: a, Status: "publish_wait"}
	child := Task{ID: "child", MissionID: root.MissionID, Team: App, Kind: "change", ContractVersion: root.ContractVersion, HeadSHA: b, Status: "publish_wait", ImageSourceTaskID: root.ID}
	acceptance := Task{ID: "accept", MissionID: root.MissionID, Team: App, Kind: "acceptance", SourceTaskID: child.ID, Status: "done"}
	st.Tasks[root.ID], st.Tasks[child.ID], st.Tasks[acceptance.ID] = root, child, acceptance
	st.ImageReleases[root.ID] = ImageReleaseRecord{SourceTaskID: root.ID, DeploymentTaskID: child.ID, Image: ImageDeploymentSpec{SourceSHA: a}}
	st.Agents["author"] = Agent{ID: "author", TaskID: child.ID, Team: App}
	st.Decisions["decision"] = ReleaseDecision{ID: "decision", AuthorAgentID: "author", Verdict: "allow", HeadSHA: b, Operation: "merge", TargetEnvironment: "private-canary", PolicyVersion: ReleasePolicyVersion, ExpiresAt: now.Add(time.Hour)}
	st.Intents["intent"] = PublishIntent{ID: "intent", DecisionID: "decision", HeadSHA: b, Status: "sent", Operation: "merge", ExternalID: c}
	st.Deployments[child.ID] = DeploymentReceipt{TaskID: child.ID, DecisionID: "decision", IntentID: "intent", HeadSHA: b, Revision: c, ImageSourceSHA: c, ObservedRelease: "v2", Status: "healthy", Environment: "private-canary", RecordedAt: now}
	return st, child, acceptance
}

func TestImageFailureCreatesFreshSourceAtDeployedCAndCompletesRoot(t *testing.T) {
	st, child, acceptance := imageCorrectionState()
	now := time.Now().UTC()
	if err := startImageCorrection(&st, child, acceptance, "accept-agent", "failure", ArtifactRef{ID: "failed-operation"}, now); err != nil {
		t.Fatal(err)
	}
	var correction Task
	for _, task := range st.Tasks {
		if task.AppRootTaskID == "root" {
			correction = task
		}
	}
	if correction.ID == "" || correction.BaseSHA != st.Deployments[child.ID].Revision || correction.HeadSHA != correction.BaseSHA || correction.ImageSourceTaskID != "" || correction.CorrectionCount != 1 || correction.Title != st.Tasks["root"].Title {
		t.Fatalf("source correction %+v", correction)
	}
	if st.Tasks[child.ID].Status != "superseded" || st.Tasks[acceptance.ID].Status != "superseded" || st.Tasks["root"].Status != "publish_wait" || st.Tasks["root"].CorrectionCount != 1 {
		t.Fatal("old flow resumed or root completed prematurely")
	}
	stages := 0
	for _, agent := range st.Agents {
		if agent.TaskID == correction.ID {
			stages++
			if agent.Status != "ready" || agent.SessionID != "" || agent.SessionGeneration != 1 {
				t.Fatal("correction reused session")
			}
		}
	}
	if stages != 5 {
		t.Fatal("new source lacks full independent stages")
	}
	nextChild := Task{ID: "child2", ImageSourceTaskID: correction.ID}
	correction.HeadSHA = strings.Repeat("d", 40)
	correction.Status = "publish_wait"
	st.Tasks[correction.ID] = correction
	st.ImageReleases[correction.ID] = ImageReleaseRecord{SourceTaskID: correction.ID, DeploymentTaskID: nextChild.ID, Image: ImageDeploymentSpec{SourceSHA: correction.HeadSHA}}
	completeImageSource(&st, nextChild, "success", now)
	if st.Tasks["root"].Status != "done" || st.Tasks[correction.ID].Status != "done" {
		t.Fatal("accepted correction did not complete original feature")
	}
}

func TestImageCorrectionRequiresDeploymentAndStopsAfterTwo(t *testing.T) {
	for _, scenario := range []string{"unknown", "changed-source", "limit"} {
		t.Run(scenario, func(t *testing.T) {
			st, child, acceptance := imageCorrectionState()
			switch scenario {
			case "unknown":
				i := st.Intents["intent"]
				i.Status = "unknown"
				st.Intents[i.ID] = i
			case "changed-source":
				r := st.Tasks["root"]
				r.HeadSHA = strings.Repeat("e", 40)
				st.Tasks[r.ID] = r
			case "limit":
				r := st.Tasks["root"]
				r.CorrectionCount = 2
				st.Tasks[r.ID] = r
			}
			err := startImageCorrection(&st, child, acceptance, "agent", "fail", ArtifactRef{}, time.Now().UTC())
			if scenario != "limit" && err == nil {
				t.Fatal("unverified correction admitted")
			}
			if scenario == "limit" && (err != nil || st.Tasks["root"].Status != "decision_wait" || st.Tasks[child.ID].Status != "decision_wait" || len(st.Tasks) != 3) {
				t.Fatal("root correction limit bypassed")
			}
		})
	}
}
