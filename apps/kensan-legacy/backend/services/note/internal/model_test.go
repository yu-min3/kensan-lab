package note

import "testing"

func TestContentType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		input ContentType
		want  bool
	}{
		{"markdown", ContentTypeMarkdown, true},
		{"drawio", ContentTypeDrawio, true},
		{"image", ContentTypeImage, true},
		{"pdf", ContentTypePDF, true},
		{"code", ContentTypeCode, true},
		{"mindmap", ContentTypeMindmap, true},
		{"empty string", ContentType(""), false},
		{"unknown type", ContentType("video"), false},
		{"case sensitive", ContentType("Markdown"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.want {
				t.Errorf("ContentType(%q).IsValid() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNoteFormat_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		input NoteFormat
		want  bool
	}{
		{"markdown", NoteFormatMarkdown, true},
		{"drawio", NoteFormatDrawio, true},
		{"empty string", NoteFormat(""), false},
		{"unknown format", NoteFormat("html"), false},
		{"case sensitive", NoteFormat("Drawio"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.want {
				t.Errorf("NoteFormat(%q).IsValid() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNoteType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		input NoteType
		want  bool
	}{
		{"diary", NoteTypeDiary, true},
		{"learning", NoteTypeLearning, true},
		{"any non-empty string", NoteType("custom"), true},
		{"empty string", NoteType(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.IsValid(); got != tt.want {
				t.Errorf("NoteType(%q).IsValid() = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
