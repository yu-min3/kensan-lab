package core

import "time"

const SchemaVersion = 1

type Team string

const (
	Platform Team = "platform"
	App      Team = "app"
)

type Task struct {
	AppRootTaskID        string    `json:"app_root_task_id,omitempty"`
	ImageSourceTaskID    string    `json:"image_source_task_id,omitempty"`
	ID                   string    `json:"id"`
	MissionID            string    `json:"mission_id"`
	Team                 Team      `json:"team"`
	Kind                 string    `json:"kind"`
	Title                string    `json:"title"`
	Status               string    `json:"status"`
	ContractVersion      string    `json:"contract_version"`
	BaseSHA              string    `json:"base_sha,omitempty"`
	HeadSHA              string    `json:"head_sha,omitempty"`
	SourceTaskID         string    `json:"source_task_id,omitempty"`
	CheckoutSourceTaskID string    `json:"checkout_source_task_id,omitempty"`
	CorrectionCount      int       `json:"correction_count,omitempty"`
	FeedbackMessageID    string    `json:"feedback_message_id,omitempty"`
	DependsOn            []string  `json:"depends_on,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Agent struct {
	ID                string        `json:"id"`
	TaskID            string        `json:"task_id"`
	Team              Team          `json:"team"`
	Role              string        `json:"role"`
	Provider          string        `json:"provider"`
	Model             string        `json:"model"`
	SessionID         string        `json:"session_id,omitempty"`
	SessionGeneration int           `json:"session_generation"`
	InputHash         string        `json:"input_hash,omitempty"`
	MemoVersion       int           `json:"memo_version"`
	Status            string        `json:"status"`
	DependsOn         []string      `json:"depends_on,omitempty"`
	AttemptCount      int           `json:"attempt_count"`
	RetryAfter        *time.Time    `json:"retry_after,omitempty"`
	ReviewAuthorID    string        `json:"review_author_id,omitempty"`
	ReviewInputs      []ArtifactRef `json:"review_inputs,omitempty"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type Attempt struct {
	ID              string       `json:"id"`
	AgentID         string       `json:"agent_id"`
	TaskID          string       `json:"task_id"`
	MissionID       string       `json:"mission_id"`
	Team            Team         `json:"team"`
	Role            string       `json:"role"`
	Provider        string       `json:"provider"`
	Model           string       `json:"model"`
	SessionID       string       `json:"session_id,omitempty"`
	Generation      int          `json:"session_generation"`
	InputHash       string       `json:"input_manifest_hash"`
	ContractVersion string       `json:"contract_version"`
	BaseSHA         string       `json:"base_sha,omitempty"`
	HeadSHA         string       `json:"head_sha,omitempty"`
	Status          string       `json:"status"`
	Reason          string       `json:"reason,omitempty"`
	OutputRef       *ArtifactRef `json:"output_ref,omitempty"`
	StartedAt       time.Time    `json:"started_at"`
	FinishedAt      *time.Time   `json:"finished_at,omitempty"`
	// LeaseExpiresAt を過ぎた running は ExpireLeases が interrupted にして
	// provider を解放する。生きている worker は RenewLease で伸ばす。
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
}

type Artifact struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	AgentID   string    `json:"agent_id"`
	Kind      string    `json:"kind"`
	Version   int       `json:"version"`
	SHA256    string    `json:"sha256"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type ArtifactRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	SHA256  string `json:"sha256"`
}

type Message struct {
	ID              string        `json:"id"`
	CorrelationID   string        `json:"correlation_id"`
	ReplyTo         string        `json:"reply_to,omitempty"`
	FromAgent       string        `json:"from_agent"`
	ToAgent         string        `json:"to_agent"`
	SourceTask      string        `json:"source_task"`
	TargetTask      string        `json:"target_task"`
	Kind            string        `json:"kind"`
	ArtifactRefs    []ArtifactRef `json:"artifact_refs"`
	ContractVersion string        `json:"contract_version"`
	HeadSHA         string        `json:"head_sha,omitempty"`
	ScenarioID      string        `json:"scenario_id,omitempty"`
	Expected        string        `json:"expected,omitempty"`
	Observed        string        `json:"observed,omitempty"`
	Status          string        `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	ReceivedAt      *time.Time    `json:"received_at,omitempty"`
}

type ContextManifest struct {
	ImageSourceSHA       string        `json:"image_source_sha,omitempty"`
	ImageSourceTaskID    string        `json:"image_source_task_id,omitempty"`
	ImageEvidence        *ArtifactRef  `json:"image_evidence,omitempty"`
	SchemaVersion        int           `json:"schema_version"`
	MissionID            string        `json:"mission_id"`
	TaskID               string        `json:"task_id"`
	SourceTaskID         string        `json:"source_task_id,omitempty"`
	CheckoutSourceTaskID string        `json:"checkout_source_task_id,omitempty"`
	Team                 Team          `json:"team"`
	Role                 string        `json:"role"`
	AgentID              string        `json:"agent_id"`
	Provider             string        `json:"provider"`
	Model                string        `json:"model"`
	Generation           int           `json:"generation"`
	TeamProfile          ArtifactRef   `json:"team_profile"`
	TeamKnowledge        ArtifactRef   `json:"team_knowledge"`
	CommonKnowledge      ArtifactRef   `json:"common_knowledge"`
	AgentMemo            *ArtifactRef  `json:"agent_memo,omitempty"`
	Inbox                []ArtifactRef `json:"inbox"`
	PreviousAcceptance   *ArtifactRef  `json:"previous_acceptance,omitempty"`
	DeploymentEvidence   *ArtifactRef  `json:"deployment_evidence,omitempty"`
	StageInputs          []StageInput  `json:"stage_inputs"`
	ReviewInputs         []ArtifactRef `json:"review_inputs"`
	ReviewAuthorID       string        `json:"review_author_id,omitempty"`
	MessageIDs           []string      `json:"message_ids"`
	ContractVersion      string        `json:"contract_version"`
	BaseSHA              string        `json:"base_sha,omitempty"`
	HeadSHA              string        `json:"head_sha,omitempty"`
	AllowedScope         []string      `json:"allowed_scope"`
	InputSHA256          string        `json:"input_sha256"`
}

type StageInput struct {
	AgentID  string      `json:"agent_id"`
	Role     string      `json:"role"`
	Artifact ArtifactRef `json:"artifact"`
}

type Event struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Subject string    `json:"subject"`
	Detail  string    `json:"detail,omitempty"`
	At      time.Time `json:"at"`
}

type ReleaseDecision struct {
	ImageDeployment   *ImageDeploymentSpec `json:"image_deployment,omitempty"`
	ImageRelease      *ImageReleaseSpec    `json:"image_release,omitempty"`
	HumanCategories   []string             `json:"human_categories,omitempty"`
	HumanReasons      []string             `json:"human_reasons,omitempty"`
	ApprovalID        string               `json:"approval_id,omitempty"`
	ID                string               `json:"id"`
	AuthorAgentID     string               `json:"author_agent_id"`
	GateAgentID       string               `json:"gate_agent_id"`
	Verdict           string               `json:"verdict"`
	Reason            string               `json:"reason"`
	Operation         string               `json:"operation"`
	Repository        string               `json:"repository"`
	Ref               string               `json:"ref"`
	HeadSHA           string               `json:"head_sha"`
	TargetEnvironment string               `json:"target_environment"`
	PolicyVersion     string               `json:"policy_version"`
	ArtifactRefs      []ArtifactRef        `json:"artifact_refs"`
	EvidenceRefs      []ArtifactRef        `json:"evidence_refs"`
	ScanRef           ArtifactRef          `json:"scan_ref"`
	SecretFree        bool                 `json:"secret_free"`
	PrivateTarget     bool                 `json:"private_target"`
	Reversible        bool                 `json:"reversible"`
	CIComplete        bool                 `json:"ci_complete"`
	CreatedAt         time.Time            `json:"created_at"`
	ExpiresAt         time.Time            `json:"expires_at"`
}

// ReleaseCandidate is operator/controller-supplied proposed action data. It
// is review input, never an authorization by itself.
type ImageReleaseSpec struct {
	SourceSHA        string `json:"source_sha"`
	SourceAppTreeSHA string `json:"source_app_tree_sha"`
	ImageTag         string `json:"image_tag"`
	WorkflowRef      string `json:"workflow_ref"`
	WorkflowSHA      string `json:"workflow_sha"`
	WorkflowPath     string `json:"workflow_path"`
	WorkflowSHA256   string `json:"workflow_sha256"`
	DispatchID       string `json:"dispatch_id"`
}

type ReleaseCandidate struct {
	ImageDeployment    *ImageDeploymentSpec `json:"image_deployment,omitempty"`
	ImageRelease       *ImageReleaseSpec    `json:"image_release,omitempty"`
	SchemaVersion      int                  `json:"schema_version"`
	Operation          string               `json:"operation"`
	Repository         string               `json:"repository"`
	Ref                string               `json:"ref"`
	HeadSHA            string               `json:"head_sha"`
	TargetEnvironment  string               `json:"target_environment"`
	Impact             string               `json:"impact"`
	Rollback           string               `json:"rollback"`
	PullRequestSummary string               `json:"pull_request_summary,omitempty"`
}

type ReleaseScan struct {
	ImageDigest      string    `json:"image_digest,omitempty"`
	ImageValuesOnly  bool      `json:"image_values_only,omitempty"`
	SourceAppTreeSHA string    `json:"source_app_tree_sha,omitempty"`
	HumanCategories  []string  `json:"human_categories,omitempty"`
	HumanReasons     []string  `json:"human_reasons,omitempty"`
	Team             Team      `json:"team,omitempty"`
	BaseSHA          string    `json:"base_sha"`
	HeadSHA          string    `json:"head_sha"`
	Repository       string    `json:"repository"`
	Ref              string    `json:"ref"`
	Operation        string    `json:"operation"`
	PolicyVersion    string    `json:"policy_version"`
	Status           string    `json:"status"`
	CommitCount      int       `json:"commit_count"`
	ChangedPaths     []string  `json:"changed_paths"`
	Findings         []string  `json:"findings"`
	DiffSHA256       string    `json:"diff_sha256"`
	ScannedAt        time.Time `json:"scanned_at"`
}

type PublishIntent struct {
	// AuthorizedAt records the live authorization check before mutation.
	AuthorizedAt       time.Time            `json:"authorized_at,omitempty"`
	ImageDeployment    *ImageDeploymentSpec `json:"image_deployment,omitempty"`
	ImageRelease       *ImageReleaseSpec    `json:"image_release,omitempty"`
	ExpiresAt          time.Time            `json:"expires_at,omitempty"`
	BaseSHA            string               `json:"base_sha,omitempty"`
	TargetEnvironment  string               `json:"target_environment,omitempty"`
	PolicyVersion      string               `json:"policy_version,omitempty"`
	ID                 string               `json:"id"`
	DecisionID         string               `json:"decision_id"`
	Operation          string               `json:"operation"`
	Repository         string               `json:"repository"`
	Ref                string               `json:"ref"`
	HeadSHA            string               `json:"head_sha"`
	Status             string               `json:"status"`
	ExternalID         string               `json:"external_id,omitempty"`
	PullRequestSummary string               `json:"pull_request_summary,omitempty"`
	UpdatedAt          time.Time            `json:"updated_at,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
}

type Question struct {
	ID              string     `json:"id"`
	TaskID          string     `json:"task_id"`
	AgentID         string     `json:"agent_id"`
	Prompt          string     `json:"prompt"`
	ContractVersion string     `json:"contract_version"`
	HeadSHA         string     `json:"head_sha,omitempty"`
	Status          string     `json:"status"`
	Answer          string     `json:"answer,omitempty"`
	ActionID        string     `json:"action_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	AnsweredAt      *time.Time `json:"answered_at,omitempty"`
}

type ApprovalRequest struct {
	HumanCategories []string   `json:"human_categories,omitempty"`
	HumanReasons    []string   `json:"human_reasons,omitempty"`
	PolicyVersion   string     `json:"policy_version,omitempty"`
	ScanSHA256      string     `json:"scan_sha256,omitempty"`
	CandidateSHA256 string     `json:"candidate_sha256,omitempty"`
	AuthorAgentID   string     `json:"author_agent_id,omitempty"`
	ID              string     `json:"id"`
	DecisionID      string     `json:"decision_id"`
	Operation       string     `json:"operation"`
	Repository      string     `json:"repository"`
	Ref             string     `json:"ref"`
	HeadSHA         string     `json:"head_sha"`
	Environment     string     `json:"environment"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	ActionID        string     `json:"action_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
}

type DailyReport struct {
	Date           string     `json:"date"`
	Status         string     `json:"status"`
	DeliveryStatus string     `json:"delivery_status"`
	Summary        string     `json:"summary"`
	TaskIDs        []string   `json:"task_ids"`
	PendingIDs     []string   `json:"pending_ids"`
	CreatedAt      time.Time  `json:"created_at"`
	SentAt         *time.Time `json:"sent_at,omitempty"`
}

// ReportOutboxEntry is the immutable scheduled snapshot plus its separately
// reconciled delivery state. A preview is never silently promoted to delivery.
type ReportOutboxEntry struct {
	Date                   string     `json:"date"`
	Status                 string     `json:"status"`
	Summary                string     `json:"summary"`
	TaskIDs                []string   `json:"task_ids"`
	PendingIDs             []string   `json:"pending_ids"`
	Destination            string     `json:"destination,omitempty"`
	AttemptID              string     `json:"attempt_id,omitempty"`
	ExternalID             string     `json:"external_id,omitempty"`
	ReconciliationEvidence string     `json:"reconciliation_evidence,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	SentAt                 *time.Time `json:"sent_at,omitempty"`
}

type DeploymentReceipt struct {
	ObservedRelease string      `json:"observed_release"`
	ImageSourceSHA  string      `json:"image_source_sha"`
	TaskID          string      `json:"task_id"`
	DecisionID      string      `json:"decision_id"`
	IntentID        string      `json:"intent_id"`
	HeadSHA         string      `json:"head_sha"`
	Revision        string      `json:"revision"`
	ImageDigest     string      `json:"image_digest"`
	Environment     string      `json:"environment"`
	Status          string      `json:"status"`
	UserPath        string      `json:"user_path"`
	EvidenceRef     ArtifactRef `json:"evidence_ref"`
	RecordedAt      time.Time   `json:"recorded_at"`
}

type FeedbackLoop struct {
	FeedbackID        string   `json:"feedback_id"`
	AnalysisTaskID    string   `json:"analysis_task_id"`
	DecisionID        string   `json:"decision_id,omitempty"`
	ImprovementTaskID string   `json:"improvement_task_id,omitempty"`
	RetestTaskID      string   `json:"retest_task_id,omitempty"`
	Status            string   `json:"status"`
	HumanCategories   []string `json:"human_categories,omitempty"`
	HumanReasons      []string `json:"human_reasons,omitempty"`
}

type State struct {
	ImageReleases map[string]ImageReleaseRecord `json:"image_releases"`
	ReleaseFlows  map[string]ReleaseFlow        `json:"release_flows"`
	Deployments   map[string]DeploymentReceipt  `json:"deployments"`
	FeedbackLoops map[string]FeedbackLoop       `json:"feedback_loops"`
	SchemaVersion int                           `json:"schema_version"`
	Tasks         map[string]Task               `json:"tasks"`
	Agents        map[string]Agent              `json:"agents"`
	Attempts      map[string]Attempt            `json:"attempts"`
	Artifacts     map[string]Artifact           `json:"artifacts"`
	Messages      map[string]Message            `json:"messages"`
	Decisions     map[string]ReleaseDecision    `json:"decisions"`
	Intents       map[string]PublishIntent      `json:"intents"`
	Questions     map[string]Question           `json:"questions"`
	Approvals     map[string]ApprovalRequest    `json:"approvals"`
	Reports       map[string]DailyReport        `json:"reports"`
	ReportOutbox  map[string]ReportOutboxEntry  `json:"report_outbox"`
	Events        []Event                       `json:"events"`
	PausedUntil   *time.Time                    `json:"paused_until,omitempty"`
	Stopped       bool                          `json:"stopped"`
}

func NewState() State {
	return State{ImageReleases: map[string]ImageReleaseRecord{}, SchemaVersion: SchemaVersion, ReleaseFlows: map[string]ReleaseFlow{}, Deployments: map[string]DeploymentReceipt{}, FeedbackLoops: map[string]FeedbackLoop{}, Tasks: map[string]Task{}, Agents: map[string]Agent{}, Attempts: map[string]Attempt{}, Artifacts: map[string]Artifact{}, Messages: map[string]Message{}, Decisions: map[string]ReleaseDecision{}, Intents: map[string]PublishIntent{}, Questions: map[string]Question{}, Approvals: map[string]ApprovalRequest{}, Reports: map[string]DailyReport{}, ReportOutbox: map[string]ReportOutboxEntry{}, Events: []Event{}}
}
