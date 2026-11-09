package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
)

func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// StorageResolver supplies trusted metadata for captures, their raw artifacts,
// and durable jobs. Extend Next for published dataset/run artifacts later.
type StorageResolver struct {
	Stores map[string]*objects.Store
	Jobs   *jobs.Store
	Next   policy.Resolver
}

func (r *StorageResolver) Lookup(ctx context.Context, p identity.Principal, typ, id string) (policy.Resource, error) {
	if typ == "capture" || (typ == "artifact" && strings.HasPrefix(id, "raw_")) {
		store, ok := r.Stores[p.AppID]
		if !ok {
			return policy.Resource{}, policy.ErrNotFound
		}
		captureID := id
		if typ == "artifact" {
			captureID = strings.TrimPrefix(id, "raw_")
		}
		c, e := store.GetCapture(ctx, captureID)
		if e != nil {
			if errors.Is(e, objects.ErrNotFound) {
				return policy.Resource{}, policy.ErrNotFound
			}
			return policy.Resource{}, policy.ErrUnavailable
		}
		subject, role, e := store.CaptureOwnership(ctx, captureID)
		if e != nil || subject == "" || role == "" {
			return policy.Resource{}, policy.ErrUnavailable
		}
		return policy.Resource{ID: id, Type: typ, AppID: c.AppID, OwnerSubject: subject, OwnerRole: role, Kind: "online"}, nil
	}
	if typ == "job" && r.Jobs != nil {
		j, e := r.Jobs.Get(jobs.Owner{App: p.AppID, Role: p.Role, Subject: p.Subject}, id)
		if e != nil {
			if errors.Is(e, jobs.ErrNotFound) {
				return policy.Resource{}, policy.ErrNotFound
			}
			return policy.Resource{}, policy.ErrUnavailable
		}
		return policy.Resource{ID: id, Type: typ, AppID: j.Admission.Owner.App, OwnerSubject: j.Admission.Owner.Subject, OwnerRole: j.Admission.Owner.Role, Kind: j.Admission.Kind}, nil
	}
	if r.Next != nil {
		return r.Next.Lookup(ctx, p, typ, id)
	}
	return policy.Resource{}, policy.ErrNotFound
}
