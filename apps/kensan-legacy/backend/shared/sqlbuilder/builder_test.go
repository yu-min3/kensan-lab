package sqlbuilder

import (
	"testing"
)

func TestUpdateBuilder_AddField(t *testing.T) {
	b := NewUpdateBuilder()

	name := "test"
	AddField(b, "name", &name)

	if !b.HasUpdates() {
		t.Error("expected HasUpdates to be true")
	}
	if b.SetClause() != "name = $1" {
		t.Errorf("expected 'name = $1', got '%s'", b.SetClause())
	}
	if len(b.Args()) != 1 || b.Args()[0] != "test" {
		t.Errorf("unexpected args: %v", b.Args())
	}
}

func TestUpdateBuilder_AddField_NilIgnored(t *testing.T) {
	b := NewUpdateBuilder()

	AddField[string](b, "name", nil)

	if b.HasUpdates() {
		t.Error("expected HasUpdates to be false for nil field")
	}
}

func TestUpdateBuilder_MultipleFields(t *testing.T) {
	b := NewUpdateBuilder()

	name := "test"
	color := "#FF0000"
	AddField(b, "name", &name)
	AddField(b, "color", &color)

	expected := "name = $1, color = $2"
	if b.SetClause() != expected {
		t.Errorf("expected '%s', got '%s'", expected, b.SetClause())
	}
	if len(b.Args()) != 2 {
		t.Errorf("expected 2 args, got %d", len(b.Args()))
	}
}

func TestUpdateBuilder_AddFieldValue(t *testing.T) {
	b := NewUpdateBuilder()
	b.AddFieldValue("status", "active")

	if b.SetClause() != "status = $1" {
		t.Errorf("expected 'status = $1', got '%s'", b.SetClause())
	}
}

func TestUpdateBuilder_AddArg(t *testing.T) {
	b := NewUpdateBuilder()
	name := "test"
	AddField(b, "name", &name)

	idArg := b.AddArg("some-id")
	userArg := b.AddArg("some-user-id")

	if idArg != 2 {
		t.Errorf("expected idArg=2, got %d", idArg)
	}
	if userArg != 3 {
		t.Errorf("expected userArg=3, got %d", userArg)
	}
	if len(b.Args()) != 3 {
		t.Errorf("expected 3 args, got %d", len(b.Args()))
	}
}

func TestWhereBuilder_Basic(t *testing.T) {
	w := NewWhereBuilder("user-123")

	if w.WhereClause() != "WHERE user_id = $1" {
		t.Errorf("unexpected clause: %s", w.WhereClause())
	}
	if len(w.Args()) != 1 || w.Args()[0] != "user-123" {
		t.Errorf("unexpected args: %v", w.Args())
	}
}

func TestWhereBuilder_AddFilter(t *testing.T) {
	w := NewWhereBuilder("user-123")

	status := "active"
	AddFilter(w, "status", &status)

	expected := "WHERE user_id = $1 AND status = $2"
	if w.WhereClause() != expected {
		t.Errorf("expected '%s', got '%s'", expected, w.WhereClause())
	}
}

func TestWhereBuilder_AddFilter_NilIgnored(t *testing.T) {
	w := NewWhereBuilder("user-123")

	AddFilter[string](w, "status", nil)

	if w.ArgCount() != 1 {
		t.Errorf("expected argCount=1, got %d", w.ArgCount())
	}
}

func TestWhereBuilder_AddInClause(t *testing.T) {
	w := NewWhereBuilder("user-123")
	w.AddInClause("type", []string{"diary", "learning"})

	expected := "WHERE user_id = $1 AND type IN ($2, $3)"
	if w.WhereClause() != expected {
		t.Errorf("expected '%s', got '%s'", expected, w.WhereClause())
	}
	if len(w.Args()) != 3 {
		t.Errorf("expected 3 args, got %d", len(w.Args()))
	}
}

func TestWhereBuilder_AddInClause_Empty(t *testing.T) {
	w := NewWhereBuilder("user-123")
	w.AddInClause("type", []string{})

	if w.ArgCount() != 1 {
		t.Errorf("expected no change, got argCount=%d", w.ArgCount())
	}
}

func TestWhereBuilder_AddLike(t *testing.T) {
	w := NewWhereBuilder("user-123")
	w.AddLike([]string{"title", "content"}, "Test")

	expected := "WHERE user_id = $1 AND (LOWER(title) LIKE $2 OR LOWER(content) LIKE $2)"
	if w.WhereClause() != expected {
		t.Errorf("expected '%s', got '%s'", expected, w.WhereClause())
	}
	if len(w.Args()) != 2 || w.Args()[1] != "%test%" {
		t.Errorf("unexpected args: %v", w.Args())
	}
}

func TestWhereBuilder_AddLike_Empty(t *testing.T) {
	w := NewWhereBuilder("user-123")
	w.AddLike([]string{"title"}, "")

	if w.ArgCount() != 1 {
		t.Errorf("expected no change for empty pattern")
	}
}

func TestWhereBuilder_AddLimit(t *testing.T) {
	w := NewWhereBuilder("user-123")
	limitStr := w.AddLimit(20)

	if limitStr != "LIMIT $2" {
		t.Errorf("expected 'LIMIT $2', got '%s'", limitStr)
	}
}

func TestWhereBuilder_AddFilterWithCast(t *testing.T) {
	w := NewWhereBuilder("user-123")

	ts := "2026-01-01T00:00:00Z"
	AddFilterWithCast(w, "start_datetime", ">=", "::timestamptz", &ts)

	expected := "WHERE user_id = $1 AND start_datetime >= $2::timestamptz"
	if w.WhereClause() != expected {
		t.Errorf("expected '%s', got '%s'", expected, w.WhereClause())
	}
}
