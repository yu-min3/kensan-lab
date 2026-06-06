package workspace

import (
	"reflect"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		head string
		want Meta
	}{
		{
			name: "standard",
			head: "---\ntype: note\ntitle: \"テスト\"\nstatus: active\ntags: [go, kensan]\ncreated: 2026-06-06\nupdated: 2026-06-06\n---\n\n## 概要\n",
			want: Meta{Type: "note", Title: "テスト", Status: "active", Tags: []string{"go", "kensan"}, Created: "2026-06-06", Updated: "2026-06-06"},
		},
		{
			name: "no frontmatter = 未分類（エラーではない）",
			head: "# ただの markdown\n本文\n",
			want: Meta{},
		},
		{
			name: "broken yaml = ParseError 付き未分類",
			head: "---\ntype: [unclosed\n---\n",
			want: Meta{}, // ParseError は別途確認
		},
		{
			name: "extra fields は Extra へ",
			head: "---\ntype: book\nauthor: \"著者\"\nrating: 5\n---\n",
			want: Meta{Type: "book", Extra: map[string]any{"author": "著者", "rating": 5}},
		},
		{
			name: "crlf",
			head: "---\r\ntype: daily\r\n---\r\n\r\n# 2026-06-06\r\n",
			want: Meta{Type: "daily"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFrontmatter([]byte(tt.head))
			if tt.name == "broken yaml = ParseError 付き未分類" {
				if got.ParseError == "" {
					t.Fatalf("want ParseError, got %+v", got)
				}
				if got.Type != "" {
					t.Fatalf("broken yaml should be uncategorized, got type %q", got.Type)
				}
				return
			}
			got.ParseError = ""
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
