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
			// /assets/* は内容ハッシュ付きなので実質不変。長期キャッシュしてよい。
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// 実体の無い /assets/* に index.html を返すと、デプロイでハッシュが変わった直後に
		// 古い index.html を掴んだブラウザが「CSS/JS として HTML を受け取る」状態になり、
		// スタイル全滅・真っ白になる。ここは素直に 404 を返す。
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			http.NotFound(w, r)
			return
		}
		// index.html は毎回検証させる（アセットのハッシュ変更に確実に追随させる）
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
