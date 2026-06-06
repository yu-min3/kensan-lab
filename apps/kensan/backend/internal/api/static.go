package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WithStatic は API にフロントエンド（ビルド済み SPA）の配信を重ねる。
// dir が空なら API のみ（ローカル開発: Vite dev server が別で動く）。
// コンテナでは KENSAN_STATIC_DIR=/srv/dist を指す（単一 image 構成）。
func WithStatic(apiHandler http.Handler, dir string) http.Handler {
	if dir == "" {
		return apiHandler
	}
	fileServer := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			apiHandler.ServeHTTP(w, r)
			return
		}
		// 実在するアセットはそのまま、それ以外は SPA fallback で index.html
		// （/tasks 等のクライアントルートを直接開いた場合）
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}
