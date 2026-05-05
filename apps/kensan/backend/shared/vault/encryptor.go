package vault

import (
	"context"
	"errors"
)

// Encryptor is the minimal interface user-service Repository depends on.
// Decoupling the dependency from *Client lets tests substitute a NoOpEncryptor
// or an in-memory fake without touching a real Vault.
type Encryptor interface {
	Encrypt(ctx context.Context, plaintext []byte) (string, error)
	Decrypt(ctx context.Context, ciphertext string) ([]byte, error)
	HMAC(ctx context.Context, input []byte) (string, error)
}

// Compile-time guarantee that *Client satisfies Encryptor.
var _ Encryptor = (*Client)(nil)

// NoOpEncryptor is a passthrough Encryptor used when Vault is not configured
// (e.g., docker-compose dev). It stores plaintext as-is so the rest of the
// stack still works.
//
// SECURITY: NoOpEncryptor MUST NOT be used in production. Callers wire it
// only when LoadConfigFromEnv() returns Address == "".
type NoOpEncryptor struct{}

// Encrypt returns the plaintext unchanged with a "noop:" prefix to make the
// fact that no encryption happened visible in the DB.
func (NoOpEncryptor) Encrypt(_ context.Context, plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}
	return "noop:" + string(plaintext), nil
}

// Decrypt strips the "noop:" prefix.
func (NoOpEncryptor) Decrypt(_ context.Context, ciphertext string) ([]byte, error) {
	if ciphertext == "" {
		return nil, ErrEmptyCiphertext
	}
	if len(ciphertext) < 5 || ciphertext[:5] != "noop:" {
		return nil, errors.New("vault: noop encryptor cannot decrypt non-noop ciphertext")
	}
	return []byte(ciphertext[5:]), nil
}

// HMAC returns a deterministic but un-keyed marker so tests can equality-match.
// It is NOT a real HMAC — just enough for tests to detect "same input → same output".
func (NoOpEncryptor) HMAC(_ context.Context, input []byte) (string, error) {
	if len(input) == 0 {
		return "", ErrEmptyPlaintext
	}
	return "noop-hmac:" + string(input), nil
}
