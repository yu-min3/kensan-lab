package workspace

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// skipDirs はスキャン対象から外すディレクトリ名。
// inner code repo は symlink で繋がれる想定のため、symlink を辿らないことでも除外される。
var skipDirs = map[string]bool{
	"node_modules": true,
	"temp":         true,
}

type cacheEntry struct {
	mtime time.Time
	size  int64
	meta  Meta
}

// Workspace は kensan-workspace ルートへの読み取りアクセス。
type Workspace struct {
	Root string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func New(root string) *Workspace {
	return &Workspace{Root: root, cache: map[string]cacheEntry{}}
}

// Scan は workspace 内の全 Markdown ドキュメントのメタ情報を返す。
// 毎回 WalkDir + stat する。frontmatter のパースだけ mtime/size キーで省略する。
func (w *Workspace) Scan() ([]Doc, error) {
	seen := map[string]bool{}
	var docs []Doc
	err := filepath.WalkDir(w.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 読めないエントリは無視（寛容設計）
		}
		name := d.Name()
		if d.IsDir() {
			if path != w.Root {
				if skipDirs[name] || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				// 入れ子の git repo（inner code repo）は workspace の一部ではない。
				// 移行後は symlink になる予定だが、実ディレクトリの間もここで除外する。
				if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // symlink は辿らない（inner repo 等）
		}
		if !strings.HasSuffix(name, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(w.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		docs = append(docs, w.doc(path, rel, info))
		return nil
	})
	// 消えたファイルのキャッシュを掃除（メモリリーク防止。正しさには影響しない）
	w.mu.Lock()
	for k := range w.cache {
		if !seen[k] {
			delete(w.cache, k)
		}
	}
	w.mu.Unlock()

	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, err
}

func (w *Workspace) doc(abs, rel string, info fs.FileInfo) Doc {
	w.mu.Lock()
	e, ok := w.cache[rel]
	w.mu.Unlock()
	if ok && e.mtime.Equal(info.ModTime()) && e.size == info.Size() {
		return Doc{Path: rel, Size: info.Size(), MTime: info.ModTime(), Meta: e.meta}
	}
	meta := parseFrontmatter(readHead(abs))
	w.mu.Lock()
	w.cache[rel] = cacheEntry{mtime: info.ModTime(), size: info.Size(), meta: meta}
	w.mu.Unlock()
	return Doc{Path: rel, Size: info.Size(), MTime: info.ModTime(), Meta: meta}
}

// Read はファイルの本文とメタ情報を返す。path は workspace 相対。
func (w *Workspace) Read(rel string) (Doc, []byte, error) {
	abs, err := w.Abs(rel)
	if err != nil {
		return Doc{}, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Doc{}, nil, err
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return Doc{}, nil, err
	}
	return w.doc(abs, filepath.ToSlash(rel), info), content, nil
}

// Abs は相対パスを検証付きで絶対パスへ解決する（path traversal 防止）。
func (w *Workspace) Abs(rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	abs := filepath.Join(w.Root, rel)
	cleanRoot := filepath.Clean(w.Root) + string(filepath.Separator)
	if !strings.HasPrefix(abs, cleanRoot) {
		return "", fs.ErrInvalid
	}
	return abs, nil
}

func readHead(abs string) []byte {
	f, err := os.Open(abs)
	if err != nil {
		return nil
	}
	defer f.Close()
	head, _ := io.ReadAll(io.LimitReader(f, headerLimit))
	return head
}
