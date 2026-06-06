package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// レビュー成果物（/weekly-review, /reflection が生成する HTML）の一覧と配信。
// ファイル契約: reviews/YYYY/WNN.html（週次）, reviews/daily/*.html（日次）,
// reviews/YYYY/MM-monthly.*（月次）。md も後方互換で列挙する。

type reviewEntry struct {
	Path  string    `json:"path"` // workspace 相対
	Name  string    `json:"name"`
	Kind  string    `json:"kind"` // weekly | daily | monthly | other
	MTime time.Time `json:"mtime"`
	Size  int64     `json:"size"`
}

var weeklyRe = regexp.MustCompile(`^W\d+`)

// GET /api/v1/reviews
func (s *Server) handleReviews(w http.ResponseWriter, _ *http.Request) {
	root := filepath.Join(s.ws.Root, "reviews")
	var out []reviewEntry
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// workspace.Scan と同じ意味論: 隠しディレクトリは見ない
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // symlink は辿らない（workspace.Scan と同じ）
		}
		if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.ws.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		kind := "other"
		switch {
		case strings.Contains(rel, "/daily/"):
			kind = "daily"
		case weeklyRe.MatchString(name):
			kind = "weekly"
		case strings.Contains(name, "monthly"):
			kind = "monthly"
		}
		out = append(out, reviewEntry{Path: rel, Name: name, Kind: kind, MTime: info.ModTime(), Size: info.Size()})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].MTime.After(out[j].MTime) })
	writeJSON(w, http.StatusOK, map[string]any{"reviews": out, "total": len(out)})
}

// GET /api/v1/reviews/{path...} — レビューファイルを生のまま配信（iframe 用）。
// reviews/ サブツリー限定。Content-Type は拡張子から（.html → text/html）。
func (s *Server) handleReviewContent(w http.ResponseWriter, r *http.Request) {
	rel := "reviews/" + r.PathValue("path")
	abs, err := s.ws.Abs(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	// Abs は workspace 外を拒否するが、reviews/ 外への .. 移動も明示的に塞ぐ
	cleanRel, err := filepath.Rel(s.ws.Root, filepath.Clean(abs))
	if err != nil || !strings.HasPrefix(filepath.ToSlash(cleanRel), "reviews/") {
		writeError(w, http.StatusBadRequest, "path must be under reviews/")
		return
	}
	// http.ServeFile は symlink を辿るため、解決後の実体も reviews/ 配下であることを検証する
	// （reviews/evil -> /etc のような symlink 経由の workspace 外配信を防ぐ）
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	realRoot, err := filepath.EvalSymlinks(filepath.Join(s.ws.Root, "reviews"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if resolved != realRoot && !strings.HasPrefix(resolved, realRoot+string(filepath.Separator)) {
		writeError(w, http.StatusBadRequest, "symlink escapes reviews/")
		return
	}
	http.ServeFile(w, r, abs)
}
