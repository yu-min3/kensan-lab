package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
	runtimeclient "github.com/yu-min3/kensan-lab/apps/llmops/internal/runtime"
)

type RuntimeGateway interface {
	Create(context.Context, jobs.Owner, runtimeclient.Request) (runtimeclient.Job, error)
	Get(context.Context, jobs.Owner, string) (runtimeclient.Job, error)
	Action(context.Context, jobs.Owner, string, string) (runtimeclient.Job, error)
	ClientID() string
	TargetID() string
}

// RuntimeRoute is operator configuration. Factory binds its client to the
// verified/persisted Owner; it must not select endpoints from request data.
type RuntimeRoute struct {
	AppID, Role, TargetID string
	Factory               func(jobs.Owner) (RuntimeGateway, error)
}
type JobConfig struct {
	Authenticate func(http.Handler) http.Handler
	Policy       *policy.Engine
	Store        *jobs.Store
	Objects      map[string]*objects.Store
	Routes       map[string]RuntimeRoute   // fixed downstream client ID -> route
	DailyLimits  map[string]map[string]int // app -> online/eval -> daily limit
}
type retryBudget struct {
	count int
	after time.Time
}
type JobAPI struct {
	auth     func(http.Handler) http.Handler
	policy   *policy.Engine
	store    *jobs.Store
	objects  map[string]*objects.Store
	routes   map[string]RuntimeRoute
	limits   map[string]map[string]int
	dispatch sync.Mutex
	mu       sync.Mutex
	clients  map[string]RuntimeGateway
	retries  map[string]retryBudget
	wake     chan struct{}
}

func NewJobs(c JobConfig) (*JobAPI, error) {
	if c.Authenticate == nil || c.Policy == nil || c.Store == nil || len(c.Routes) == 0 {
		return nil, errors.New("job authentication, policy, store and routes required")
	}
	h := &JobAPI{auth: c.Authenticate, policy: c.Policy, store: c.Store, objects: map[string]*objects.Store{}, routes: map[string]RuntimeRoute{}, limits: map[string]map[string]int{}, clients: map[string]RuntimeGateway{}, retries: map[string]retryBudget{}, wake: make(chan struct{}, 1)}
	for id, r := range c.Routes {
		if id == "" || r.AppID == "" || (r.Role != "app" && r.Role != "eval") || r.TargetID == "" || r.Factory == nil {
			return nil, errors.New("invalid runtime route")
		}
		h.routes[id] = r
	}
	for app, limits := range c.DailyLimits {
		h.limits[app] = map[string]int{}
		for kind, n := range limits {
			if (kind != "online" && kind != "eval") || n < 0 {
				return nil, errors.New("invalid daily budget")
			}
			h.limits[app][kind] = n
		}
	}
	for app, s := range c.Objects {
		if s == nil || s.AppID() != app {
			return nil, errors.New("invalid app object store")
		}
		h.objects[app] = s
	}
	for _, route := range h.routes {
		if h.objects[route.AppID] == nil {
			return nil, errors.New("runtime app object store required")
		}
	}
	pending, e := h.store.Pending()
	if e != nil {
		return nil, e
	}
	for _, record := range pending {
		if _, e := h.gateway(record.Admission.Owner, record.Admission.RuntimeClient, record.Admission.RuntimeTarget); e != nil {
			return nil, errors.New("pending runtime route unavailable")
		}
	}
	return h, nil
}
func (h *JobAPI) Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/jobs", h.auth(http.HandlerFunc(h.createJob)))
	mux.Handle("GET /v1/jobs", h.auth(http.HandlerFunc(h.listJobs)))
	mux.Handle("GET /v1/jobs/{id}", h.auth(http.HandlerFunc(h.getJob)))
	mux.Handle("POST /v1/jobs/{id}/cancel", h.auth(http.HandlerFunc(h.actionJob)))
	mux.Handle("POST /v1/jobs/{id}/retry", h.auth(http.HandlerFunc(h.actionJob)))
	mux.Handle("GET /v1/artifacts/{id}", h.auth(http.HandlerFunc(h.artifact)))
}
func owner(p identity.Principal) jobs.Owner {
	return jobs.Owner{App: p.AppID, Role: p.Role, Subject: p.Subject}
}
func (h *JobAPI) gateway(o jobs.Owner, clientID, target string) (RuntimeGateway, error) {
	route, ok := h.routes[clientID]
	if !ok || route.AppID != o.App || route.Role != o.Role || route.TargetID != target {
		return nil, policy.ErrForbidden
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	key := o.App + "\x00" + o.Role + "\x00" + o.Subject + "\x00" + clientID
	if cached, ok := h.clients[key]; ok {
		return cached, nil
	}
	gateway, e := route.Factory(o)
	if e != nil {
		return nil, e
	}
	if gateway.ClientID() != clientID || gateway.TargetID() != target {
		return nil, policy.ErrForbidden
	}
	h.clients[key] = gateway
	return gateway, nil
}
func jobError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, jobs.ErrNotFound), errors.Is(e, policy.ErrNotFound):
		writeError(w, 404, "not_found")
	case errors.Is(e, policy.ErrForbidden):
		writeError(w, 403, "forbidden")
	case errors.Is(e, jobs.ErrConflict), errors.Is(e, jobs.ErrInvalid):
		writeError(w, 409, "conflict")
	case errors.Is(e, jobs.ErrQuota):
		writeError(w, 429, "quota_exceeded")
	default:
		writeError(w, 503, "unavailable")
	}
}
func (h *JobAPI) wakeDispatch(id string) {
	h.mu.Lock()
	delete(h.retries, id)
	h.mu.Unlock()
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

var idempotencyPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,128}$`)

func (h *JobAPI) createJob(w http.ResponseWriter, r *http.Request) {
	p, _ := Principal(r)
	if !idempotencyPattern.MatchString(r.Header.Get("Idempotency-Key")) {
		writeError(w, 400, "invalid_request")
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, 400, "invalid_request")
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, runtimeclient.MaxRequestBytes))
	if e != nil {
		writeError(w, 413, "too_large")
		return
	}
	body, e := decodeObject(raw, "job-request")
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	scope, e := h.policy.AuthorizeCreate(r.Context(), p, policy.JobCreate, body)
	if e != nil {
		jobError(w, e)
		return
	}
	route, ok := h.routes[p.RuntimeClient]
	if !ok || route.AppID != p.AppID || route.Role != p.Role {
		writeError(w, 403, "forbidden")
		return
	}
	if _, e = h.gateway(owner(p), p.RuntimeClient, route.TargetID); e != nil {
		jobError(w, e)
		return
	}
	if len(body["prompt"].(string)) > 32768 {
		writeError(w, 413, "too_large")
		return
	}
	outputSchema, _ := json.Marshal(body["output_schema"])
	input, _ := json.Marshal(body["input"])
	downstream, _ := json.Marshal(runtimeclient.Request{Profile: "text", Prompt: body["prompt"].(string), Input: input, OutputSchema: outputSchema, IdempotencyKey: strings.Repeat("x", 39)})
	if len(outputSchema) > 32768 || len(downstream) > runtimeclient.MaxRequestBytes {
		writeError(w, 413, "too_large")
		return
	}
	canonical, _ := json.Marshal(body)
	record, _, e := h.store.Admit(jobs.Admission{Owner: owner(p), Key: r.Header.Get("Idempotency-Key"), Kind: scope.Kind, Payload: canonical, RuntimeClient: p.RuntimeClient, RuntimeTarget: route.TargetID}, h.limits[p.AppID][scope.Kind])
	if e != nil {
		jobError(w, e)
		return
	}
	h.wakeDispatch(record.ID)
	writeJSON(w, 202, publicJob(record, nil))
}
func publicJob(r jobs.Record, j *runtimeclient.Job) map[string]any {
	result := map[string]any{"id": r.ID, "state": r.State, "kind": r.Admission.Kind, "attempt_id": r.ID, "created_at": r.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": r.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	if r.Phase == "uncertain" || r.Phase == "reconciling" {
		result["state"] = "waiting"
		result["reason"] = "dispatch_reconciliation_required"
	}
	if r.Reason != "" {
		reason := r.Reason
		if len(reason) > 256 {
			reason = "upstream_reason_unavailable"
		}
		result["reason"] = reason
	}
	if j != nil && len(j.Output) > 0 && r.State == "succeeded" {
		result["output"] = j.Output
	}
	return result
}
func (h *JobAPI) authorizedJob(r *http.Request, action string) (identity.Principal, jobs.Record, error) {
	p, _ := Principal(r)
	id := r.PathValue("id")
	if _, e := h.policy.AuthorizeResource(r.Context(), p, action, id); e != nil {
		return p, jobs.Record{}, e
	}
	record, e := h.store.Get(owner(p), id)
	return p, record, e
}
func (h *JobAPI) poll(ctx context.Context, record jobs.Record) (jobs.Record, *runtimeclient.Job, error) {
	if record.Phase == "terminal" && record.RuntimeJobID != "" {
		p := identity.Principal{AppID: record.Admission.Owner.App, Role: record.Admission.Owner.Role, Subject: record.Admission.Owner.Subject}
		saved, err := h.result(ctx, p, "job_"+record.ID)
		if err == nil {
			return record, &runtimeclient.Job{ID: record.RuntimeJobID, State: record.State, Reason: record.Reason, Output: saved.Output, Model: saved.Model, Usage: saved.Usage}, nil
		}
		if !errors.Is(err, policy.ErrNotFound) {
			return record, nil, err
		}
	}

	if record.RuntimeJobID == "" {
		return record, nil, nil
	}
	client, e := h.gateway(record.Admission.Owner, record.Admission.RuntimeClient, record.Admission.RuntimeTarget)
	if e != nil {
		return record, nil, e
	}
	j, e := client.Get(ctx, record.Admission.Owner, record.RuntimeJobID)
	if e != nil {
		return record, nil, e
	}
	updated, e := h.store.Link(record.Admission.Owner, record.ID, j.ID, j.State, j.Reason)
	if errors.Is(e, jobs.ErrInvalid) {
		latest, err := h.store.Get(record.Admission.Owner, record.ID)
		return latest, nil, err
	}
	if e != nil {
		return record, nil, e
	}
	if j.State == "succeeded" || j.State == "failed" || j.State == "cancelled" {
		if e = h.saveResult(ctx, updated, j); e != nil {
			return updated, nil, e
		}
		p := identity.Principal{AppID: updated.Admission.Owner.App, Role: updated.Admission.Owner.Role, Subject: updated.Admission.Owner.Subject}
		saved, e := h.result(ctx, p, "job_"+updated.ID)
		if e != nil {
			return updated, nil, e
		}
		j.Output = saved.Output
		j.Model = saved.Model
		j.Usage = saved.Usage
	}
	return updated, &j, nil
}
func (h *JobAPI) getJob(w http.ResponseWriter, r *http.Request) {
	_, record, e := h.authorizedJob(r, policy.JobRead)
	if e != nil {
		jobError(w, e)
		return
	}
	record, j, e := h.poll(r.Context(), record)
	if e != nil {
		jobError(w, e)
		return
	}
	writeJSON(w, 200, publicJob(record, j))
}
func (h *JobAPI) listJobs(w http.ResponseWriter, r *http.Request) {
	p, _ := Principal(r)
	scope, e := h.policy.ListScope(p, policy.JobList)
	if e != nil {
		jobError(w, e)
		return
	}
	after := ""
	for key, values := range r.URL.Query() {
		if (key != "cursor" && key != "limit") || len(values) != 1 {
			writeError(w, 400, "invalid_request")
			return
		}
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		after, e = h.policy.ReadCursor(p, cursor)
		if e != nil {
			jobError(w, e)
			return
		}
	}
	limit := 50
	if text := r.URL.Query().Get("limit"); text != "" {
		limit, e = strconv.Atoi(text)
		if e != nil || limit < 1 || limit > 100 {
			writeError(w, 400, "invalid_request")
			return
		}
	}
	records, e := h.store.List(owner(p))
	if e != nil {
		jobError(w, e)
		return
	}
	// Stable ID ordering matches signed after ID and never exposes global totals.
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	visible := []jobs.Record{}
	for _, record := range records {
		if record.Admission.Kind == scope.Kind && record.ID > after {
			visible = append(visible, record)
		}
	}
	results := []map[string]any{}
	next := ""
	for i, record := range visible {
		if i == limit {
			next, _ = h.policy.Cursor(p, visible[i-1].ID)
			break
		}
		results = append(results, publicJob(record, nil))
	}
	page := map[string]any{"items": results}
	if next != "" {
		page["next_cursor"] = next
	}
	writeJSON(w, 200, page)
}
func (h *JobAPI) actionJob(w http.ResponseWriter, r *http.Request) {
	action := policy.JobCancel
	if strings.HasSuffix(r.URL.Path, "/retry") {
		action = policy.JobRetry
	}
	p, record, e := h.authorizedJob(r, action)
	if e != nil {
		jobError(w, e)
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if e != nil {
		writeError(w, 413, "too_large")
		return
	}
	if len(raw) > 0 {
		var body map[string]any
		if json.Unmarshal(raw, &body) != nil || body == nil || len(body) > 0 {
			writeError(w, 400, "invalid_request")
			return
		}
	}
	h.dispatch.Lock()
	defer h.dispatch.Unlock()
	record, e = h.store.Get(owner(p), record.ID)
	if e != nil {
		jobError(w, e)
		return
	}
	if action == policy.JobRetry && record.State == "failed" {
		if !idempotencyPattern.MatchString(r.Header.Get("Idempotency-Key")) {
			writeError(w, 400, "invalid_request")
			return
		}
		record, _, e = h.store.Admit(jobs.Admission{Owner: record.Admission.Owner, Key: "retry:" + record.ID + ":" + hashBytes([]byte(r.Header.Get("Idempotency-Key"))), Kind: record.Admission.Kind, Payload: record.Admission.Payload, RuntimeClient: record.Admission.RuntimeClient, RuntimeTarget: record.Admission.RuntimeTarget}, h.limits[p.AppID][record.Admission.Kind])
		if e != nil {
			jobError(w, e)
			return
		}
		h.wakeDispatch(record.ID)
		writeJSON(w, 202, publicJob(record, nil))
		return
	}
	if record.Phase == "prepared" && action == policy.JobCancel {
		record, e = h.store.CancelPrepared(owner(p), record.ID)
		if e != nil {
			jobError(w, e)
			return
		}
		writeJSON(w, 200, publicJob(record, nil))
		return
	}
	if action == policy.JobRetry && (record.State == "waiting" || record.Phase == "uncertain" || record.Phase == "reconciling") && record.RuntimeJobID == "" {
		h.wakeDispatch(record.ID)
		writeJSON(w, 200, publicJob(record, nil))
		return
	}
	if record.RuntimeJobID == "" {
		writeError(w, 409, "conflict")
		return
	}
	client, e := h.gateway(owner(p), record.Admission.RuntimeClient, record.Admission.RuntimeTarget)
	if e != nil {
		jobError(w, e)
		return
	}
	cmd := "cancel"
	if action == policy.JobRetry {
		if record.State != "waiting" {
			writeError(w, 409, "conflict")
			return
		}
		cmd = "retry"
	}
	j, e := client.Action(r.Context(), owner(p), record.RuntimeJobID, cmd)
	if e != nil {
		var re *runtimeclient.Error
		if errors.As(e, &re) && re.Status == 409 {
			writeError(w, 409, "conflict")
			return
		}
		jobError(w, e)
		return
	}
	record, e = h.store.Link(owner(p), record.ID, j.ID, j.State, j.Reason)
	if e != nil {
		jobError(w, e)
		return
	}
	writeJSON(w, 200, publicJob(record, &j))
}

// Run retries uncertain Create using its original persisted key. At most three
// automatic transport attempts are made per process; a replay of the same
// upper request explicitly enables another bounded reconciliation cycle.
func (h *JobAPI) Run(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-h.wake:
		}
		if e := h.DispatchOnce(ctx); e != nil && ctx.Err() != nil {
			return nil
		}
	}
}
func (h *JobAPI) DispatchOnce(ctx context.Context) error {
	h.dispatch.Lock()
	defer h.dispatch.Unlock()
	pending, e := h.store.Pending()
	if e != nil {
		return e
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return pending[i].Admission.Kind == "online" && pending[j].Admission.Kind != "online"
	})
	for _, record := range pending {
		h.mu.Lock()
		budget := h.retries[record.ID]
		h.mu.Unlock()
		if budget.count >= 3 || time.Now().Before(budget.after) {
			continue
		}
		client, e := h.gateway(record.Admission.Owner, record.Admission.RuntimeClient, record.Admission.RuntimeTarget)
		if e != nil {
			return e
		}
		record, e = h.store.BeginDispatch(record.Admission.Owner, record.ID)
		if e != nil {
			return e
		}
		var body struct {
			Profile      string          `json:"profile"`
			Prompt       string          `json:"prompt"`
			Input        json.RawMessage `json:"input"`
			OutputSchema json.RawMessage `json:"output_schema"`
		}
		if json.Unmarshal(record.Admission.Payload, &body) != nil {
			return jobs.ErrInvalid
		}
		j, e := client.Create(ctx, record.Admission.Owner, runtimeclient.Request{Profile: body.Profile, Prompt: body.Prompt, Input: body.Input, OutputSchema: body.OutputSchema, IdempotencyKey: record.DownstreamKey})
		if e != nil {
			state, reason := "", ""
			var rejected *runtimeclient.Error
			if errors.As(e, &rejected) {
				if rejected.Kind == "authentication_unavailable" {
					state, reason = "waiting", "runtime_auth_required"
				}
				switch rejected.Status {
				case 401, 403:
					state, reason = "waiting", "runtime_auth_required"
				case 429:
					state, reason = "waiting", "runtime_quota_wait"
				case 400, 404, 405, 409, 413, 415, 422:
					state, reason = "failed", "runtime_request_rejected"
				}
			}
			if state != "" {
				if _, err := h.store.RejectDispatch(record.Admission.Owner, record.ID, state, reason); err != nil {
					return err
				}
			}
			h.mu.Lock()
			h.retries[record.ID] = retryBudget{3, time.Now().Add(time.Second)}
			if state == "" {
				h.retries[record.ID] = retryBudget{budget.count + 1, time.Now().Add(time.Second * time.Duration(1<<budget.count))}
			}
			h.mu.Unlock()
			return e
		}
		_, e = h.store.Link(record.Admission.Owner, record.ID, j.ID, j.State, j.Reason)
		return e
	}
	return nil
}
