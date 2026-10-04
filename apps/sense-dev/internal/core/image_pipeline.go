package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

// ImageDeploymentSpec is host-verified CI provenance, carried unchanged into
// each fresh Gate for the GitOps candidate. SourceSHA is A, never B or C.
type ImageDeploymentSpec struct {
	WorkflowRef      string `json:"workflow_ref"`
	SourceSHA        string `json:"source_sha"`
	SourceAppTreeSHA string `json:"source_app_tree_sha"`
	ImageTag         string `json:"image_tag"`
	Digest           string `json:"digest"`
	WorkflowSHA      string `json:"workflow_sha"`
	WorkflowSHA256   string `json:"workflow_sha256"`
	DispatchID       string `json:"dispatch_id"`
	WorkflowRunID    int64  `json:"workflow_run_id"`
}

type ImageReleaseRecord struct {
	SourceTaskID     string              `json:"source_task_id"`
	DecisionID       string              `json:"decision_id"`
	IntentID         string              `json:"intent_id"`
	Image            ImageDeploymentSpec `json:"image"`
	EvidenceRef      ArtifactRef         `json:"evidence_ref"`
	DeploymentTaskID string              `json:"deployment_task_id,omitempty"`
	RecordedAt       time.Time           `json:"recorded_at"`
}

func ValidateImageDeploymentSpec(p ImageDeploymentSpec) error {
	if !workflowRefPattern.MatchString(p.WorkflowRef) || !githubCommitPattern.MatchString(p.SourceSHA) || !githubCommitPattern.MatchString(p.SourceAppTreeSHA) || !githubCommitPattern.MatchString(p.WorkflowSHA) || !fullDigest(p.WorkflowSHA256) || !dispatchIDPattern.MatchString(p.DispatchID) || p.ImageTag != "sense-"+p.SourceSHA+"-"+p.DispatchID || !imageDigestPattern.MatchString(p.Digest) || p.WorkflowRunID <= 0 {
		return errors.New("deployment image needs complete, immutable CI provenance")
	}
	return ValidateImageReleaseSpec(ImageReleaseSpec{SourceSHA: p.SourceSHA, SourceAppTreeSHA: p.SourceAppTreeSHA, ImageTag: p.ImageTag, WorkflowRef: p.WorkflowRef, WorkflowSHA: p.WorkflowSHA, WorkflowPath: CanaryImageWorkflowPath, WorkflowSHA256: p.WorkflowSHA256, DispatchID: p.DispatchID})
}

func cloneImageDeployment(p *ImageDeploymentSpec) *ImageDeploymentSpec {
	if p == nil {
		return nil
	}
	copy := *p
	return &copy
}
func imageDeploymentsEqual(a, b *ImageDeploymentSpec) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// RecordImageRelease admits only the authenticated publisher's verified CI
// observation, never a worker-produced artifact or a successful dispatch alone.
func (s *Store) RecordImageRelease(r ImageReleaseRecord) error {
	if err := ValidateImageDeploymentSpec(r.Image); err != nil {
		return err
	}
	if err := s.verifyRef(r.EvidenceRef); err != nil {
		return err
	}
	artifact := s.Snapshot().Artifacts[r.EvidenceRef.ID]
	if artifact.AgentID != "system" || artifact.Kind != "image_release_observation" {
		return errors.New("host image evidence required")
	}
	body, err := s.ReadArtifact(artifact.ID)
	if err != nil {
		return err
	}
	var observed ImageReleaseRecord
	if json.Unmarshal(body, &observed) != nil || observed.SourceTaskID != r.SourceTaskID || observed.DecisionID != r.DecisionID || observed.IntentID != r.IntentID || observed.Image != r.Image {
		return errors.New("image observation identity mismatch")
	}
	return s.update(func(st *State) error {
		source, ok := st.Tasks[r.SourceTaskID]
		d, okD := st.Decisions[r.DecisionID]
		i, okI := st.Intents[r.IntentID]
		if !ok || !okD || !okI || source.Team != App || source.Kind != "change" || source.Status != "publish_wait" || source.ImageSourceTaskID != "" || source.HeadSHA != r.Image.SourceSHA || d.Verdict != "allow" || d.Operation != "image_publish" || d.PolicyVersion != ReleasePolicyVersion || st.Agents[d.AuthorAgentID].TaskID != source.ID || i.DecisionID != d.ID || i.Status != "sent" || !publishAuthorizationMatches(*st, i, d) || i.Operation != "image_publish" || i.HeadSHA != source.HeadSHA || i.TargetEnvironment != "private-canary" || !imageReleasesEqual(i.ImageRelease, d.ImageRelease) || i.ImageRelease == nil {
			return errors.New("image is not the source task's sent gated operation")
		}
		p := i.ImageRelease
		if p.SourceSHA != r.Image.SourceSHA || p.SourceAppTreeSHA != r.Image.SourceAppTreeSHA || p.ImageTag != r.Image.ImageTag || p.WorkflowSHA != r.Image.WorkflowSHA || p.WorkflowSHA256 != r.Image.WorkflowSHA256 || p.WorkflowRef != r.Image.WorkflowRef || p.DispatchID != r.Image.DispatchID || i.ExternalID != p.DispatchID {
			return errors.New("CI image differs from authorized dispatch")
		}
		if old, exists := st.ImageReleases[source.ID]; exists && old.Image.SourceSHA == source.HeadSHA {
			if old.Image == r.Image && old.IntentID == r.IntentID {
				return nil
			}
			return errors.New("conflicting image release")
		}
		r.RecordedAt = time.Now().UTC()
		r.DeploymentTaskID = ""
		st.ImageReleases[source.ID] = r
		st.Events = append(st.Events, event("image_release_verified", source.ID, r.Image.Digest))
		return nil
	})
}

// ReconcileImageDeployments creates one durable values-only child with the
// source's original base. Each of its five stages and release Gates is new.
func (s *Store) ReconcileImageDeployments(mission string) (int, error) {
	st := s.Snapshot()
	if st.Stopped || st.PausedUntil != nil && time.Now().Before(*st.PausedUntil) {
		return 0, nil
	}
	var ids []string
	for id, r := range st.ImageReleases {
		if r.DeploymentTaskID == "" && st.Tasks[id].MissionID == mission {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	count := 0
	for _, id := range ids {
		childID, err := newID()
		if err != nil {
			return count, err
		}
		stages, _ := defaultStages("change")
		agentIDs := make([]string, len(stages))
		for n := range agentIDs {
			agentIDs[n], err = newID()
			if err != nil {
				return count, err
			}
		}
		created := false
		err = s.update(func(st *State) error {
			r := st.ImageReleases[id]
			source := st.Tasks[id]
			if r.DeploymentTaskID != "" {
				return nil
			}
			if source.Team != App || source.Kind != "change" || source.Status != "publish_wait" || source.HeadSHA != r.Image.SourceSHA || !fullSHA(source.BaseSHA) || source.ImageSourceTaskID != "" {
				return errors.New("image source changed before child reservation")
			}
			now := time.Now().UTC()
			child := Task{ID: childID, MissionID: source.MissionID, Team: App, Kind: "change", Title: "配備digestを固定: " + source.Title, Status: "ready", ContractVersion: source.ContractVersion, BaseSHA: source.BaseSHA, HeadSHA: source.HeadSHA, ImageSourceTaskID: source.ID, CreatedAt: now, UpdatedAt: now}
			st.Tasks[child.ID] = child
			for n, stage := range stages {
				a := Agent{ID: agentIDs[n], TaskID: childID, Team: App, Role: stage.role, Provider: stage.provider, Model: stage.model, SessionGeneration: 1, Status: "ready", UpdatedAt: now.Add(time.Duration(n) * time.Nanosecond)}
				if n > 0 {
					a.DependsOn = append([]string(nil), agentIDs[:n]...)
				}
				st.Agents[a.ID] = a
			}
			titles := []string{}
			for oldID, old := range st.Tasks {
				if old.Team == App && old.Kind == "acceptance" && old.SourceTaskID == source.ID {
					if old.BaseSHA != "" {
						return errors.New("source acceptance already pinned before image deployment")
					}
					titles = append(titles, old.Title)
					old.Status = "superseded"
					old.UpdatedAt = now
					st.Tasks[oldID] = old
					for aid, a := range st.Agents {
						if a.TaskID == oldID {
							a.Status = "superseded"
							st.Agents[aid] = a
						}
					}
				}
			}
			if len(titles) == 0 {
				titles = append(titles, source.Title)
			}
			for _, title := range titles {
				acceptanceID, e := newID()
				if e != nil {
					return e
				}
				agentID, e := newID()
				if e != nil {
					return e
				}
				st.Tasks[acceptanceID] = Task{ID: acceptanceID, MissionID: source.MissionID, Team: App, Kind: "acceptance", Title: title, Status: "ready", ContractVersion: source.ContractVersion, SourceTaskID: childID, CreatedAt: now, UpdatedAt: now}
				st.Agents[agentID] = Agent{ID: agentID, TaskID: acceptanceID, Team: App, Role: "app_acceptance", Provider: "claude", Model: "opus", SessionGeneration: 1, Status: "ready", UpdatedAt: now}
			}
			r.DeploymentTaskID = childID
			st.ImageReleases[id] = r
			st.Events = append(st.Events, event("image_deployment_planned", childID, id))
			created = true
			return nil
		})
		if err != nil {
			return count, err
		}
		if created {
			count++
		}
	}
	return count, nil
}

func deploymentImageForTask(st State, task Task) *ImageDeploymentSpec {
	if task.ImageSourceTaskID != "" {
		r, ok := st.ImageReleases[task.ImageSourceTaskID]
		source := st.Tasks[task.ImageSourceTaskID]
		if ok && r.DeploymentTaskID == task.ID && source.HeadSHA == r.Image.SourceSHA && source.BaseSHA == task.BaseSHA && source.MissionID == task.MissionID && source.ContractVersion == task.ContractVersion {
			return cloneImageDeployment(&r.Image)
		}
	}
	if task.Team == Platform && task.CheckoutSourceTaskID != "" {
		source := st.Tasks[task.CheckoutSourceTaskID]
		receipt := st.Deployments[source.ID]
		intent := st.Intents[receipt.IntentID]
		if appDeploymentReady(st, source) && task.BaseSHA == receipt.Revision {
			return cloneImageDeployment(intent.ImageDeployment)
		}
	}
	return nil
}

func (s *Store) validateDeploymentCandidate(authorTask Task, c ReleaseCandidate, scan ReleaseScan) error {
	expected := deploymentImageForTask(s.Snapshot(), authorTask)
	if expected == nil && c.ImageDeployment == nil {
		return nil
	}
	if c.ImageDeployment == nil || expected == nil || !imageDeploymentsEqual(c.ImageDeployment, expected) || ValidateImageDeploymentSpec(*expected) != nil || c.TargetEnvironment != "private-canary" || c.Operation == "image_publish" || scan.SourceAppTreeSHA != expected.SourceAppTreeSHA || scan.ImageDigest != expected.Digest {
		return errors.New("deployment candidate differs from host image or actual Git tree/values")
	}
	if authorTask.ImageSourceTaskID != "" && !scan.ImageValuesOnly {
		return errors.New("image deployment child modified more than image values")
	}
	return nil
}

// A deployment child may only change the image mapping after A. Intermediate
// source or policy edits cannot disappear from the final tree and bypass review.
func imageOnlyValuesChange(before, after []byte) bool {
	a, err := strictValues(before)
	if err != nil {
		return false
	}
	b, err := strictValues(after)
	if err != nil {
		return false
	}
	delete(a, "image")
	delete(b, "image")
	return reflect.DeepEqual(a, b)
}
