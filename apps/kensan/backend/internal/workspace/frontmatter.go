package workspace

import (
	"bytes"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// headerLimit はメタデータ取得時にファイル先頭から読む最大バイト数。
// notes/sessions/ のような巨大ファイルでも frontmatter はこの範囲に収まる。
const headerLimit = 64 * 1024

// parseFrontmatter はファイル先頭チャンクから frontmatter を抽出する。
// いかなる失敗もエラーにせず、ParseError を埋めた Meta を返す（寛容設計）。
func parseFrontmatter(head []byte) Meta {
	src, ok := frontmatterBlock(head)
	if !ok {
		return Meta{} // frontmatter なし = 未分類。エラーではない
	}
	var raw map[string]any
	if err := yaml.Unmarshal(src, &raw); err != nil {
		return Meta{ParseError: err.Error()}
	}
	m := Meta{}
	for k, v := range raw {
		switch k {
		case "type":
			m.Type = toString(v)
		case "title":
			m.Title = toString(v)
		case "status":
			m.Status = toString(v)
		case "tags":
			m.Tags = toStrings(v)
		case "created":
			m.Created = toString(v)
		case "updated":
			m.Updated = toString(v)
		default:
			if m.Extra == nil {
				m.Extra = map[string]any{}
			}
			m.Extra[k] = v
		}
	}
	return m
}

// frontmatterBlock は "---" で囲まれた YAML ブロックを取り出す。
func frontmatterBlock(head []byte) ([]byte, bool) {
	var body []byte
	switch {
	case bytes.HasPrefix(head, []byte("---\n")):
		body = head[4:]
	case bytes.HasPrefix(head, []byte("---\r\n")):
		body = head[5:]
	default:
		return nil, false
	}
	for _, delim := range []string{"\n---\n", "\n---\r\n", "\r\n---\r\n", "\r\n---\n"} {
		if i := bytes.Index(body, []byte(delim)); i >= 0 {
			return body[:i], true
		}
	}
	// 閉じデリミタがファイル末尾の場合
	for _, delim := range []string{"\n---", "\r\n---"} {
		if bytes.HasSuffix(bytes.TrimRight(body, "\r\n"), []byte("---")) {
			if i := bytes.LastIndex(body, []byte(delim)); i >= 0 {
				return body[:i], true
			}
		}
	}
	return nil, false
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case time.Time:
		// yaml.v3 は `2026-06-06` のような日付を time.Time に解決する
		return t.Format("2006-01-02")
	default:
		return fmt.Sprint(v)
	}
}

func toStrings(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, toString(e))
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}
