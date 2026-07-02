// Package workspace は kensan-workspace の Markdown ファイル群への読み取りアクセスを提供する。
//
// 設計原則: インデックスを持たない。リクエスト毎に WalkDir + stat し、
// frontmatter のパース結果だけを mtime/size キーでメモ化する。
// 正しさは常に stat に立脚し、キャッシュは速度だけを担う。
package workspace

import "time"

// Meta は frontmatter から抽出したメタデータ。
// パースに失敗しても Doc は返る（ParseError が入り、Type は空 = 未分類）。
type Meta struct {
	Type       string         `json:"type,omitempty"`
	Title      string         `json:"title,omitempty"`
	Status     string         `json:"status,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Created    string         `json:"created,omitempty"`
	Updated    string         `json:"updated,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
	ParseError string         `json:"parseError,omitempty"`
}

// Doc は 1 つの Markdown ファイルのメタ情報。
type Doc struct {
	Path  string    `json:"path"` // workspace ルートからの相対パス（slash 区切り）
	Size  int64     `json:"size"`
	MTime time.Time `json:"mtime"`
	Meta  Meta      `json:"meta"`
}

// Uncategorized は frontmatter が無い・壊れている等で type が判定できないドキュメントかを返す。
func (d Doc) Uncategorized() bool { return d.Meta.Type == "" }
