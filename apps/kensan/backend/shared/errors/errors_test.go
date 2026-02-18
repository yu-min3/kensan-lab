package errors

import (
	"errors"
	"testing"
)

// Test generic NotFound constructor
func TestNotFoundConstructor(t *testing.T) {
	entities := []string{"task", "goal", "user", "note", "memo"}

	for _, entity := range entities {
		t.Run(entity, func(t *testing.T) {
			err := NotFound(entity)
			if !errors.Is(err, ErrNotFound) {
				t.Errorf("NotFound(%q) error = %v, want error wrapping %v", entity, err, ErrNotFound)
			}
			if !IsNotFound(err) {
				t.Errorf("IsNotFound(NotFound(%q)) = false, want true", entity)
			}
		})
	}
}

// Test generic AlreadyExists constructor
func TestAlreadyExistsConstructor(t *testing.T) {
	entities := []string{"user", "tag", "note"}

	for _, entity := range entities {
		t.Run(entity, func(t *testing.T) {
			err := AlreadyExists(entity)
			if !errors.Is(err, ErrAlreadyExists) {
				t.Errorf("AlreadyExists(%q) error = %v, want error wrapping %v", entity, err, ErrAlreadyExists)
			}
			if !IsAlreadyExists(err) {
				t.Errorf("IsAlreadyExists(AlreadyExists(%q)) = false, want true", entity)
			}
		})
	}
}

// Test InvalidStatus helper
func TestInvalidStatus(t *testing.T) {
	err := InvalidStatus("task")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("InvalidStatus() error = %v, want error wrapping %v", err, ErrInvalidInput)
	}
	if !IsInvalidInput(err) {
		t.Errorf("IsInvalidInput(InvalidStatus()) = false, want true")
	}
}

// Test validation format errors
func TestValidationFormatErrors(t *testing.T) {
	t.Run("ErrInvalidDate", func(t *testing.T) {
		if !errors.Is(ErrInvalidDate, ErrInvalidFormat) {
			t.Errorf("ErrInvalidDate should wrap ErrInvalidFormat")
		}
	})

	t.Run("ErrInvalidTime", func(t *testing.T) {
		if !errors.Is(ErrInvalidTime, ErrInvalidFormat) {
			t.Errorf("ErrInvalidTime should wrap ErrInvalidFormat")
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		err := InvalidFormat("date", "YYYY-MM-DD")
		if !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("InvalidFormat() error = %v, want error wrapping %v", err, ErrInvalidFormat)
		}
		if !IsInvalidFormat(err) {
			t.Errorf("IsInvalidFormat(InvalidFormat()) = false, want true")
		}
	})
}

// Test Required constructor
func TestRequired(t *testing.T) {
	err := Required("email")
	if !errors.Is(err, ErrRequired) {
		t.Errorf("Required() error = %v, want error wrapping %v", err, ErrRequired)
	}
	if !IsRequired(err) {
		t.Errorf("IsRequired(Required()) = false, want true")
	}
}

// Test EntityError type
func TestEntityError(t *testing.T) {
	err := &EntityError{Entity: "test", Base: ErrNotFound}

	t.Run("Error message", func(t *testing.T) {
		want := "test not found"
		if got := err.Error(); got != want {
			t.Errorf("EntityError.Error() = %v, want %v", got, want)
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		if got := err.Unwrap(); got != ErrNotFound {
			t.Errorf("EntityError.Unwrap() = %v, want %v", got, ErrNotFound)
		}
	})
}

// Test FieldError type
func TestFieldError(t *testing.T) {
	err := &FieldError{Field: "email", Base: ErrRequired}

	t.Run("Error message", func(t *testing.T) {
		want := "email required"
		if got := err.Error(); got != want {
			t.Errorf("FieldError.Error() = %v, want %v", got, want)
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		if got := err.Unwrap(); got != ErrRequired {
			t.Errorf("FieldError.Unwrap() = %v, want %v", got, ErrRequired)
		}
	})
}
