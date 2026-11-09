// Package dataset publishes immutable releases after table and Raw retention pins.
package dataset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/catalog"
	"regexp"
	"sort"
	"time"
)

var ErrInvalid = errors.New("invalid dataset release")
var ErrPinned = errors.New("retention pin prevents removal")
var id = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,99}$`)
var snapshot = regexp.MustCompile(`^[0-9]{1,20}$`)

type Case struct {
	SchemaVersion     string            `json:"schema_version"`
	AppID             string            `json:"app_id"`
	CaseID            string            `json:"case_id"`
	Input             objects.Reference `json:"input"`
	Expected          json.RawMessage   `json:"expected_constraints"`
	AnnotationVersion string            `json:"annotation_version"`
	AnnotationStatus  string            `json:"annotation_status"`
	Split             string            `json:"split"`
	SourceGroup       string            `json:"source_group"`
}
type Entry struct {
	RecordID string `json:"record_id"`
	Case     Case   `json:"case"`
}
type Spec struct {
	SourceTable    string  `json:"source_table"`
	DatasetTable   string  `json:"dataset_table"`
	ReleaseID      string  `json:"release_id"`
	TaskID         string  `json:"task_id"`
	SourceSnapshot string  `json:"source_snapshot"`
	CodeVersion    string  `json:"code_version"`
	AdapterVersion string  `json:"adapter_version"`
	Entries        []Entry `json:"entries"`
}
type Table struct {
	Table        string `json:"table"`
	SnapshotID   string `json:"snapshot_id"`
	RetentionTag string `json:"retention_tag"`
}
type Release struct {
	SchemaVersion  string              `json:"schema_version"`
	AppID          string              `json:"app_id"`
	ReleaseID      string              `json:"release_id"`
	TaskID         string              `json:"task_id"`
	Status         string              `json:"status"`
	Tables         []Table             `json:"tables"`
	Raw            []objects.Reference `json:"raw"`
	CaseListSHA256 string              `json:"case_list_sha256"`
	CodeVersion    string              `json:"code_version"`
	AdapterVersion string              `json:"adapter_version"`
	CreatedAt      time.Time           `json:"created_at"`
}
type Prepared struct {
	Text          string          `json:"text"`
	Content       json.RawMessage `json:"content"`
	OutputSchema  json.RawMessage `json:"output_schema"`
	TextTruncated bool            `json:"text_truncated"`
}
type Loaded struct {
	Case     Case     `json:"case"`
	Prepared Prepared `json:"prepared"`
	RecordID string   `json:"record_id"`
}

func canonical(v any) []byte {
	b, _ := json.Marshal(v)
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var x any
	_ = d.Decode(&x)
	b, _ = json.Marshal(x)
	return b
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func valid(schema string, v any) bool {
	var x any
	if json.Unmarshal(canonical(v), &x) != nil {
		return false
	}
	return contracts.Validate(schema, x) == nil
}
func Validate(app string, s Spec) error {
	if !id.MatchString(s.ReleaseID) || !id.MatchString(s.TaskID) || !snapshot.MatchString(s.SourceSnapshot) || s.CodeVersion == "" || s.AdapterVersion == "" || len(s.Entries) == 0 || len(s.Entries) > 100 {
		return ErrInvalid
	}
	groups := map[string]string{}
	seen := map[string]bool{}
	for _, entry := range s.Entries {
		c := entry.Case
		if !valid("dataset-case", c) || c.AppID != app || c.Input.AppID != app || seen[c.CaseID] || len(entry.RecordID) != 64 {
			return ErrInvalid
		}
		seen[c.CaseID] = true
		if split, ok := groups[c.SourceGroup]; ok && split != c.Split {
			return ErrInvalid
		}
		groups[c.SourceGroup] = c.Split
	}
	return nil
}
func key(app, release, file string) string {
	return "apps/" + app + "/releases/" + release + "/" + file
}
func rawPin(app, object string) string {
	return "apps/" + app + "/releases/pins/raw/" + object + ".json"
}
func immutable(ctx context.Context, b objects.Backend, k string, body []byte) error {
	e := b.PutIfAbsent(ctx, k, body, "application/json")
	if e != nil && !errors.Is(e, objects.ErrExists) {
		return e
	}
	old, e := b.Get(ctx, k, objects.MaxRecordBytes)
	if e != nil {
		return e
	}
	if !bytes.Equal(old.Bytes, body) {
		return objects.ErrConflict
	}
	return nil
}
func verifyRaw(ctx context.Context, store *objects.Store, ref objects.Reference) error {
	if ref.AppID != store.AppID() {
		return ErrInvalid
	}
	body, e := store.GetArtifact(ctx, ref.ObjectID)
	if e != nil {
		return e
	}
	if int64(len(body.Bytes)) != ref.Size || digest(body.Bytes) != ref.SHA256 {
		return objects.ErrCorrupt
	}
	return nil
}
func Build(ctx context.Context, r *worker.Runner, cfg worker.Config, b objects.Backend, s Spec) (Release, error) {
	var out Release
	e := r.WithLock("dataset-publish", func() error { var inner error; out, inner = build(ctx, r, cfg, b, s); return inner })
	return out, e
}
func build(ctx context.Context, r *worker.Runner, cfg worker.Config, b objects.Backend, s Spec) (Release, error) {
	var out Release
	if r.Role() != "transform" || b.AppID() != r.App() || cfg.Catalog == nil || cfg.DatasetSource == nil || Validate(r.App(), s) != nil {
		return out, ErrInvalid
	}
	source := *cfg.Catalog
	source.Namespace = cfg.DatasetSource.Namespace
	source.Table = cfg.DatasetSource.Table
	if catalog.Validate(&source, r.App(), r.Role()) != nil {
		return out, ErrInvalid
	}
	sourceName := r.App() + "." + source.Namespace + "." + source.Table
	targetName := r.App() + "." + cfg.Catalog.Namespace + "." + cfg.Catalog.Table
	if s.SourceTable != "" && s.SourceTable != sourceName || s.DatasetTable != "" && s.DatasetTable != targetName {
		return out, ErrInvalid
	}
	s.SourceTable = sourceName
	s.DatasetTable = targetName
	sort.Slice(s.Entries, func(i, j int) bool { return s.Entries[i].Case.CaseID < s.Entries[j].Case.CaseID })
	request := canonical(s)
	if int64(len(request)) > objects.MaxRecordBytes {
		return out, ErrInvalid
	}
	// Reserve immutable request first; conflicting reuse cannot mutate tables or pins.
	if e := immutable(ctx, b, key(r.App(), s.ReleaseID, "request.json"), request); e != nil {
		return out, e
	}
	if old, e := b.Get(ctx, key(r.App(), s.ReleaseID, "manifest.json"), objects.MaxRecordBytes); e == nil {
		if json.Unmarshal(old.Bytes, &out) != nil || !valid("dataset-release", out) {
			return out, ErrInvalid
		}
		_, e = Read(ctx, r, cfg, b, s.ReleaseID)
		return out, e
	} else if !errors.Is(e, objects.ErrNotFound) {
		return out, e
	}
	store, e := objects.NewStore(r.App(), b)
	if e != nil {
		return out, e
	}
	refs := map[string]objects.Reference{}
	cases := []Case{}
	for _, entry := range s.Entries {
		if e = verifyRaw(ctx, store, entry.Case.Input); e != nil {
			return out, e
		}
		refs[entry.Case.Input.ObjectID] = entry.Case.Input
		cases = append(cases, entry.Case)
	}
	result, e := catalog.Dataset(ctx, r, cfg.Catalog, "build", map[string]any{"release_id": s.ReleaseID, "source_namespace": source.Namespace, "source_table": source.Table, "source_snapshot": s.SourceSnapshot, "entries": s.Entries})
	if e != nil {
		return out, e
	}
	var saved struct {
		SnapshotID string `json:"snapshot_id"`
		Tag        string `json:"retention_tag"`
		Count      int    `json:"count"`
	}
	if json.Unmarshal(result, &saved) != nil || saved.Count != len(cases) || !snapshot.MatchString(saved.SnapshotID) {
		return out, ErrInvalid
	}
	raw := []objects.Reference{}
	for _, ref := range refs {
		if e = immutable(ctx, b, rawPin(r.App(), ref.ObjectID), canonical(ref)); e != nil {
			return out, e
		}
		raw = append(raw, ref)
	}
	sort.Slice(raw, func(i, j int) bool { return raw[i].ObjectID < raw[j].ObjectID })
	out = Release{SchemaVersion: "1", AppID: r.App(), ReleaseID: s.ReleaseID, TaskID: s.TaskID, Status: "published", Tables: []Table{{r.App() + "." + source.Namespace + "." + source.Table, s.SourceSnapshot, saved.Tag}, {r.App() + "." + cfg.Catalog.Namespace + "." + cfg.Catalog.Table, saved.SnapshotID, saved.Tag}}, Raw: raw, CaseListSHA256: digest(canonical(cases)), CodeVersion: s.CodeVersion, AdapterVersion: s.AdapterVersion, CreatedAt: time.Now().UTC()}
	if !valid("dataset-release", out) {
		return out, ErrInvalid
	}
	if e = immutable(ctx, b, key(r.App(), s.ReleaseID, "manifest.json"), canonical(out)); e != nil {
		return out, e
	}
	return out, nil
}
func Read(ctx context.Context, r *worker.Runner, cfg worker.Config, b objects.Backend, releaseID string) ([]Loaded, error) {
	if !id.MatchString(releaseID) || b.AppID() != r.App() || cfg.Catalog == nil {
		return nil, ErrInvalid
	}
	blob, e := b.Get(ctx, key(r.App(), releaseID, "manifest.json"), objects.MaxRecordBytes)
	if e != nil {
		return nil, e
	}
	var m Release
	if json.Unmarshal(blob.Bytes, &m) != nil || !valid("dataset-release", m) || m.AppID != r.App() || m.ReleaseID != releaseID || len(m.Tables) != 2 {
		return nil, ErrInvalid
	}
	reservation, e := b.Get(ctx, key(r.App(), releaseID, "request.json"), objects.MaxRecordBytes)
	if e != nil {
		return nil, e
	}
	var spec Spec
	if json.Unmarshal(reservation.Bytes, &spec) != nil || Validate(r.App(), spec) != nil || spec.ReleaseID != releaseID || spec.TaskID != m.TaskID || spec.CodeVersion != m.CodeVersion || spec.AdapterVersion != m.AdapterVersion {
		return nil, ErrInvalid
	}
	planned := []Case{}
	for _, entry := range spec.Entries {
		planned = append(planned, entry.Case)
	}
	if digest(canonical(planned)) != m.CaseListSHA256 {
		return nil, objects.ErrCorrupt
	}
	wanted := r.App() + "." + cfg.Catalog.Namespace + "." + cfg.Catalog.Table
	if spec.DatasetTable != wanted || spec.SourceTable == wanted {
		return nil, ErrInvalid
	}
	sourceFound := false
	targetFound := false
	for _, ref := range m.Tables {
		if ref.RetentionTag != "release_"+releaseID {
			return nil, ErrInvalid
		}
		switch ref.Table {
		case spec.SourceTable:
			if ref.SnapshotID != spec.SourceSnapshot || sourceFound {
				return nil, ErrInvalid
			}
			sourceFound = true
		case wanted:
			if targetFound {
				return nil, ErrInvalid
			}
			targetFound = true
		default:
			return nil, ErrInvalid
		}
	}
	if !sourceFound || !targetFound {
		return nil, ErrInvalid
	}
	var snap string
	for _, ref := range m.Tables {
		if ref.Table == wanted {
			snap = ref.SnapshotID
		}
	}
	if snap == "" {
		return nil, ErrInvalid
	}
	raw, e := catalog.Dataset(ctx, r, cfg.Catalog, "read", map[string]any{"release_id": releaseID, "snapshot_id": snap, "tables": m.Tables})
	if e != nil {
		return nil, e
	}
	var result struct {
		Cases []Loaded `json:"cases"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return nil, ErrInvalid
	}
	cases := []Case{}
	store, e := objects.NewStore(r.App(), b)
	if e != nil {
		return nil, e
	}
	refs := map[string]objects.Reference{}
	for _, ref := range m.Raw {
		refs[ref.ObjectID] = ref
	}
	for _, entry := range result.Cases {
		if !valid("dataset-case", entry.Case) || entry.Case.AppID != r.App() {
			return nil, ErrInvalid
		}
		ref, ok := refs[entry.Case.Input.ObjectID]
		if !ok || ref != entry.Case.Input {
			return nil, ErrInvalid
		}
		if e = verifyRaw(ctx, store, ref); e != nil {
			return nil, e
		}
		pin, e := b.Get(ctx, rawPin(r.App(), ref.ObjectID), objects.MaxRecordBytes)
		if e != nil || !bytes.Equal(pin.Bytes, canonical(ref)) {
			return nil, ErrPinned
		}
		cases = append(cases, entry.Case)
	}
	if digest(canonical(cases)) != m.CaseListSHA256 {
		return nil, objects.ErrCorrupt
	}
	return result.Cases, nil
}

// No unpin/GC is provided in V0; maintenance must reject these retained objects.
func GuardRawRemoval(ctx context.Context, b objects.Backend, ref objects.Reference) error {
	if b.AppID() != ref.AppID {
		return ErrInvalid
	}
	_, e := b.Get(ctx, rawPin(ref.AppID, ref.ObjectID), objects.MaxRecordBytes)
	if e == nil {
		return ErrPinned
	}
	if errors.Is(e, objects.ErrNotFound) {
		return nil
	}
	return e
}
func GuardSnapshotRemoval(ctx context.Context, r *worker.Runner, c *worker.CatalogConfig, id string) error {
	if !snapshot.MatchString(id) {
		return ErrInvalid
	}
	b, e := catalog.Dataset(ctx, r, c, "guard", map[string]any{"snapshot_id": id})
	if e != nil {
		return e
	}
	var result struct {
		Pinned bool `json:"pinned"`
	}
	if json.Unmarshal(b, &result) != nil {
		return ErrInvalid
	}
	if result.Pinned {
		return ErrPinned
	}
	return nil
}
