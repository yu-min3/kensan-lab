package api

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
)

// decodeJSON applies the same request boundary to every JSON write endpoint.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "application/json required", 415)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host {
			http.Error(w, "origin rejected", 403)
			return false
		}
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		http.Error(w, "入力を確認してください（形式またはサイズが不正です）", 400)
		return false
	}
	return true
}
