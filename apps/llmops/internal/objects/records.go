package objects

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
)

const MaxRecordBytes int64 = 2 << 20

// ImmutableRecord is an app-scoped JSON publication. RequestHash binds its
// immutable request; Data may additionally contain server-generated metadata.
type ImmutableRecord struct {
	Version     int             `json:"version"`
	AppID       string          `json:"app_id"`
	Collection  string          `json:"collection"`
	ID          string          `json:"id"`
	RequestHash string          `json:"request_hash"`
	Data        json.RawMessage `json:"data"`
	DataSHA256  string          `json:"data_sha256"`
}

func (s *Store) recordKey(collection, id string) (string, error) {
	if !keyID(collection) || !keyID(id) {
		return "", ErrInvalid
	}
	return "apps/" + s.app + "/records/" + collection + "/" + id + ".json", nil
}
func (s *Store) GetRecord(ctx context.Context, collection, id string) (ImmutableRecord, error) {
	key, e := s.recordKey(collection, id)
	if e != nil {
		return ImmutableRecord{}, e
	}
	blob, e := s.backend.Get(ctx, key, MaxRecordBytes)
	if e != nil {
		return ImmutableRecord{}, e
	}
	var record ImmutableRecord
	d := json.NewDecoder(bytes.NewReader(blob.Bytes))
	d.DisallowUnknownFields()
	if blob.MIME != "application/json" || d.Decode(&record) != nil || d.Decode(new(any)) != io.EOF || record.Version != 1 || record.AppID != s.app || record.Collection != collection || record.ID != id || len(record.RequestHash) != 64 || !json.Valid(record.Data) || record.DataSHA256 != sum(record.Data) {
		return ImmutableRecord{}, ErrCorrupt
	}
	return record, nil
}

// PublishRecord preserves the first Data for an identical immutable request.
// This lets timestamps be assigned by the server without breaking safe replay.
func (s *Store) PublishRecord(ctx context.Context, collection, id string, request, data json.RawMessage) (ImmutableRecord, bool, error) {
	key, e := s.recordKey(collection, id)
	if e != nil || !json.Valid(request) || !json.Valid(data) {
		return ImmutableRecord{}, false, ErrInvalid
	}
	if int64(len(request)) > MaxRecordBytes || int64(len(data)) > MaxRecordBytes {
		return ImmutableRecord{}, false, ErrTooLarge
	}
	request = canonicalRecordJSON(request)
	data = canonicalRecordJSON(data)
	digest := sum(request)
	old, e := s.GetRecord(ctx, collection, id)
	if e == nil {
		if old.RequestHash != digest {
			return ImmutableRecord{}, false, ErrConflict
		}
		return old, true, nil
	}
	if e != ErrNotFound {
		return ImmutableRecord{}, false, e
	}
	record := ImmutableRecord{Version: 1, AppID: s.app, Collection: collection, ID: id, RequestHash: digest, Data: data, DataSHA256: sum(data)}
	body, e := json.Marshal(record)
	if e != nil {
		return ImmutableRecord{}, false, ErrInvalid
	}
	if int64(len(body)) > MaxRecordBytes {
		return ImmutableRecord{}, false, ErrTooLarge
	}
	e = s.backend.PutIfAbsent(ctx, key, body, "application/json")
	if e != nil && e != ErrExists {
		return ImmutableRecord{}, false, e
	}
	published, readErr := s.GetRecord(ctx, collection, id)
	if readErr != nil {
		return ImmutableRecord{}, false, readErr
	}
	if published.RequestHash != digest {
		return ImmutableRecord{}, false, ErrConflict
	}
	return published, e == ErrExists, nil
}

func canonicalRecordJSON(raw json.RawMessage) json.RawMessage {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value any
	_ = d.Decode(&value)
	encoded, _ := json.Marshal(value)
	return encoded
}
