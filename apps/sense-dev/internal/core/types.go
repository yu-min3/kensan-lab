package core

import "time"

const SchemaVersion = 1

type Team string

const (
	Platform Team = "platform"
	App      Team = "app"
)

type Task struct {
	ID              string    `json:"id"`
	MissionID       string    `json:"mission_id"`
	Team            Team      `json:"team"`
	Kind            string    `json:"kind"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	ContractVersion string    `json:"contract_version"`
	BaseSHA         string    `json:"base_sha,omitempty"`
	HeadSHA         string    `json:"head_sha,omitempty"`
	DependsOn       []string  `json:"depends_on,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Agent struct {
	ID                string     `json:"id"`
	TaskID            string     `json:"task_id"`
	Team              Team       `json:"team"`
	Role              string     `json:"role"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	SessionID         string     `json:"session_id,omitempty"`
	SessionGeneration int        `json:"session_generation"`
	InputHash         string     `json:"input_hash,omitempty"`
	MemoVersion       int        `json:"memo_version"`
	Status            string     `json:"status"`
	DependsOn         []string   `json:"depends_on,omitempty"`
	AttemptCount      int        `json:"attempt_count"`
	RetryAfter        *time.Time `json:"retry_after,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
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
	SchemaVersion   int           `json:"schema_version"`
	MissionID       string        `json:"mission_id"`
	TaskID          string        `json:"task_id"`
	Team            Team          `json:"team"`
	Role            string        `json:"role"`
	AgentID         string        `json:"agent_id"`
	Provider        string        `json:"provider"`
	Model           string        `json:"model"`
	Generation      int           `json:"generation"`
	TeamProfile     ArtifactRef   `json:"team_profile"`
	TeamKnowledge   ArtifactRef   `json:"team_knowledge"`
	CommonKnowledge ArtifactRef   `json:"common_knowledge"`
	AgentMemo       *ArtifactRef  `json:"agent_memo,omitempty"`
	Inbox           []ArtifactRef `json:"inbox"`
	MessageIDs      []string      `json:"message_ids"`
	ContractVersion string        `json:"contract_version"`
	BaseSHA         string        `json:"base_sha,omitempty"`
	HeadSHA         string        `json:"head_sha,omitempty"`
	AllowedScope    []string      `json:"allowed_scope"`
	InputSHA256     string        `json:"input_sha256"`
}

type Event struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Subject string    `json:"subject"`
	Detail  string    `json:"detail,omitempty"`
	At      time.Time `json:"at"`
}

type ReleaseDecision struct {
	ID                string        `json:"id"`
	AuthorAgentID     string        `json:"author_agent_id"`
	GateAgentID       string        `json:"gate_agent_id"`
	Verdict           string        `json:"verdict"`
	Reason            string        `json:"reason"`
	Operation         string        `json:"operation"`
	Repository        string        `json:"repository"`
	Ref               string        `json:"ref"`
	HeadSHA           string        `json:"head_sha"`
	TargetEnvironment string        `json:"target_environment"`
	PolicyVersion     string        `json:"policy_version"`
	ArtifactRefs      []ArtifactRef `json:"artifact_refs"`
	EvidenceRefs      []ArtifactRef `json:"evidence_refs"`
	SecretFree        bool          `json:"secret_free"`
	PrivateTarget     bool          `json:"private_target"`
	Reversible        bool          `json:"reversible"`
	CIComplete        bool          `json:"ci_complete"`
	CreatedAt         time.Time     `json:"created_at"`
	ExpiresAt         time.Time     `json:"expires_at"`
}

type PublishIntent struct {
	ID         string    `json:"id"`
	DecisionID string    `json:"decision_id"`
	Operation  string    `json:"operation"`
	Repository string    `json:"repository"`
	Ref        string    `json:"ref"`
	HeadSHA    string    `json:"head_sha"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
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
	ID          string     `json:"id"`
	DecisionID  string     `json:"decision_id"`
	Operation   string     `json:"operation"`
	Repository  string     `json:"repository"`
	Ref         string     `json:"ref"`
	HeadSHA     string     `json:"head_sha"`
	Environment string     `json:"environment"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	ActionID    string     `json:"action_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
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

type State struct {
	SchemaVersion int                        `json:"schema_version"`
	Tasks         map[string]Task            `json:"tasks"`
	Agents        map[string]Agent           `json:"agents"`
	Attempts      map[string]Attempt         `json:"attempts"`
	Artifacts     map[string]Artifact        `json:"artifacts"`
	Messages      map[string]Message         `json:"messages"`
	Decisions     map[string]ReleaseDecision `json:"decisions"`
	Intents       map[string]PublishIntent   `json:"intents"`
	Questions     map[string]Question        `json:"questions"`
	Approvals     map[string]ApprovalRequest `json:"approvals"`
	Reports       map[string]DailyReport     `json:"reports"`
	Events        []Event                    `json:"events"`
	PausedUntil   *time.Time                 `json:"paused_until,omitempty"`
	Stopped       bool                       `json:"stopped"`
}

func NewState() State {
	return State{SchemaVersion: SchemaVersion, Tasks: map[string]Task{}, Agents: map[string]Agent{}, Attempts: map[string]Attempt{}, Artifacts: map[string]Artifact{}, Messages: map[string]Message{}, Decisions: map[string]ReleaseDecision{}, Intents: map[string]PublishIntent{}, Questions: map[string]Question{}, Approvals: map[string]ApprovalRequest{}, Reports: map[string]DailyReport{}, Events: []Event{}}
}
