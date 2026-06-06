package vault

import (
	"context"
	"errors"
	"testing"
)

func TestNoOpEncryptor_RoundTrip(t *testing.T) {
	enc := NoOpEncryptor{}
	ctx := context.Background()

	cases := []string{
		"Yu Misaki",
		"日本語の名前",
		"a",
	}
	for _, plain := range cases {
		t.Run(plain, func(t *testing.T) {
			ct, err := enc.Encrypt(ctx, []byte(plain))
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if ct == plain {
				t.Fatalf("Encrypt should not return raw plaintext, got %q", ct)
			}
			pt, err := enc.Decrypt(ctx, ct)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if string(pt) != plain {
				t.Fatalf("round trip mismatch: got %q want %q", pt, plain)
			}
		})
	}
}

func TestNoOpEncryptor_EmptyInputs(t *testing.T) {
	enc := NoOpEncryptor{}
	ctx := context.Background()
	if _, err := enc.Encrypt(ctx, nil); !errors.Is(err, ErrEmptyPlaintext) {
		t.Fatalf("Encrypt(nil): want ErrEmptyPlaintext, got %v", err)
	}
	if _, err := enc.Decrypt(ctx, ""); !errors.Is(err, ErrEmptyCiphertext) {
		t.Fatalf("Decrypt(empty): want ErrEmptyCiphertext, got %v", err)
	}
	if _, err := enc.HMAC(ctx, nil); !errors.Is(err, ErrEmptyPlaintext) {
		t.Fatalf("HMAC(nil): want ErrEmptyPlaintext, got %v", err)
	}
}

func TestNoOpEncryptor_HMAC_Deterministic(t *testing.T) {
	enc := NoOpEncryptor{}
	ctx := context.Background()
	a, _ := enc.HMAC(ctx, []byte("Yu Misaki"))
	b, _ := enc.HMAC(ctx, []byte("Yu Misaki"))
	if a != b {
		t.Fatalf("HMAC should be deterministic, got %q vs %q", a, b)
	}
	c, _ := enc.HMAC(ctx, []byte("Other"))
	if a == c {
		t.Fatalf("HMAC should differ for different input")
	}
}

func TestNoOpEncryptor_DecryptInvalidPrefix(t *testing.T) {
	enc := NoOpEncryptor{}
	ctx := context.Background()
	_, err := enc.Decrypt(ctx, "vault:v1:abc")
	if err == nil {
		t.Fatalf("Decrypt of non-noop ciphertext should fail")
	}
}

func TestB64RoundTrip(t *testing.T) {
	in := []byte("Yu Misaki")
	out, err := b64Decode(b64Encode(in))
	if err != nil {
		t.Fatalf("b64Decode: %v", err)
	}
	if string(out) != string(in) {
		t.Fatalf("round trip: got %q want %q", out, in)
	}
}

func TestLoadConfigFromEnv_EmptyAddress(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_AUTH_ROLE", "")
	t.Setenv("VAULT_TRANSIT_KEY", "")
	cfg := LoadConfigFromEnv()
	if cfg.Address != "" {
		t.Fatalf("Address should be empty, got %q", cfg.Address)
	}
	// MountPath should default to transit even when address is empty
	if cfg.MountPath != "transit" {
		t.Fatalf("MountPath default broken: got %q", cfg.MountPath)
	}
	if cfg.AuthMountPath != "kubernetes" {
		t.Fatalf("AuthMountPath default broken: got %q", cfg.AuthMountPath)
	}
}

func TestNewClient_NotConfigured(t *testing.T) {
	_, err := NewClient(context.Background(), Config{Address: "", KeyName: "users-name"})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}
