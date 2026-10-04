package core

import (
	"errors"
	"time"
)

// A failed feature must be fixed as App source, not smuggled into B's
// values-only task. A new source starts at the actually deployed merge C.
func startImageCorrection(st *State, child, acceptance Task, fromAgent, messageID string, evidence ArtifactRef, now time.Time) error {
	record, ok := st.ImageReleases[child.ImageSourceTaskID]
	source := st.Tasks[child.ImageSourceTaskID]
	if !ok || record.DeploymentTaskID != child.ID || source.HeadSHA != record.Image.SourceSHA || !appDeploymentReady(*st, child) {
		return errors.New("correction lacks current deployed image lineage")
	}
	rootID := source.AppRootTaskID
	if rootID == "" {
		rootID = source.ID
	}
	root, ok := st.Tasks[rootID]
	if !ok || root.Team != App || root.Kind != "change" || root.MissionID != child.MissionID || root.ContractVersion != child.ContractVersion || root.Status != "publish_wait" {
		return errors.New("correction root no longer active")
	}
	if root.CorrectionCount >= 2 {
		child.Status, child.UpdatedAt = "decision_wait", now
		root.Status, root.UpdatedAt = "decision_wait", now
		acceptance.Status, acceptance.UpdatedAt = "decision_wait", now
		st.Tasks[child.ID], st.Tasks[root.ID], st.Tasks[acceptance.ID] = child, root, acceptance
		st.Events = append(st.Events, event("correction_limit_reached", root.ID, messageID))
		return nil
	}
	id, err := newID()
	if err != nil {
		return err
	}
	stages, _ := defaultStages("change")
	ids := make([]string, len(stages))
	for n := range ids {
		ids[n], err = newID()
		if err != nil {
			return err
		}
	}
	root.CorrectionCount++
	root.UpdatedAt = now
	st.Tasks[root.ID] = root
	revision := st.Deployments[child.ID].Revision
	next := Task{ID: id, MissionID: child.MissionID, Team: App, Kind: "change", Title: root.Title, Status: "ready", ContractVersion: child.ContractVersion, BaseSHA: revision, HeadSHA: revision, AppRootTaskID: root.ID, CorrectionCount: root.CorrectionCount, CreatedAt: now, UpdatedAt: now}
	st.Tasks[id] = next
	for n, spec := range stages {
		a := Agent{ID: ids[n], TaskID: id, Team: App, Role: spec.role, Provider: spec.provider, Model: spec.model, SessionGeneration: 1, Status: "ready", UpdatedAt: now.Add(time.Duration(n) * time.Nanosecond)}
		if n > 0 {
			a.DependsOn = append([]string(nil), ids[:n]...)
		}
		st.Agents[a.ID] = a
	}
	correctionID := "image-correction-" + messageID
	st.Messages[correctionID] = Message{ID: correctionID, CorrelationID: child.MissionID, FromAgent: fromAgent, ToAgent: ids[0], SourceTask: acceptance.ID, TargetTask: id, Kind: "acceptance_failed", ArtifactRefs: []ArtifactRef{evidence}, ContractVersion: child.ContractVersion, HeadSHA: child.HeadSHA, Status: "received", CreatedAt: now, ReceivedAt: &now}
	child.Status, child.UpdatedAt = "superseded", now
	acceptance.Status, acceptance.UpdatedAt = "superseded", now
	st.Tasks[child.ID], st.Tasks[acceptance.ID] = child, acceptance
	if source.ID != root.ID {
		source.Status, source.UpdatedAt = "superseded", now
		st.Tasks[source.ID] = source
	}
	st.Events = append(st.Events, event("image_source_correction_started", id, root.ID))
	return nil
}

func completeImageSource(st *State, child Task, messageID string, now time.Time) {
	r := st.ImageReleases[child.ImageSourceTaskID]
	source := st.Tasks[child.ImageSourceTaskID]
	if r.DeploymentTaskID != child.ID || source.HeadSHA != r.Image.SourceSHA || source.Status != "publish_wait" {
		return
	}
	source.Status, source.UpdatedAt = "done", now
	st.Tasks[source.ID] = source
	st.Events = append(st.Events, event("app_change_completed", source.ID, messageID))
	if source.AppRootTaskID != "" {
		root := st.Tasks[source.AppRootTaskID]
		if root.Team == App && root.Kind == "change" && root.Status == "publish_wait" && root.MissionID == source.MissionID && root.ContractVersion == source.ContractVersion && root.CorrectionCount == source.CorrectionCount {
			root.Status, root.UpdatedAt = "done", now
			st.Tasks[root.ID] = root
			st.Events = append(st.Events, event("app_change_completed", root.ID, messageID))
		}
	}
}
