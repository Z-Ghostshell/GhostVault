package auth

import (
	"context"

	"github.com/google/uuid"
)

// AuthMode is the vault-level choice of credential style. Stored in vaults.auth_mode
// and immutable after init. See docs/UNLOCK-AND-BEARER.md.
type AuthMode string

const (
	AuthModeSession    AuthMode = "session"
	AuthModeAutoUnlock AuthMode = "auto_unlock"
	AuthModeAPIKeys    AuthMode = "api_keys"
)

// IsValidAuthMode returns true for the three modes accepted by the CHECK constraint.
// api_keys is reserved for a future resolver; callers must decide separately whether
// they want to accept it at init time (Part 3 of the plan implements it).
func IsValidAuthMode(s string) bool {
	switch AuthMode(s) {
	case AuthModeSession, AuthModeAutoUnlock, AuthModeAPIKeys:
		return true
	}
	return false
}

// Credential is the interface every resolver returns. It collapses the three auth
// modes behind one shape so api/handlers.go never branches on mode.
type Credential interface {
	VaultID() uuid.UUID
	// DEK is nil when the vault has encryption=off. Otherwise the resolver fills it
	// from VaultState (session / auto_unlock) or unwraps per-request (api_keys).
	DEK() []byte
	Scopes() []string
	// TokenHash is intended for audit logging; the raw bearer never leaves the
	// resolver.
	TokenHash() []byte
}

// CredentialResolver is the hook requireVaultAuth calls with each request's Bearer.
// One resolver is instantiated per gvsvd process based on the vault's auth_mode.
type CredentialResolver interface {
	// Resolve validates a raw bearer and returns the matching Credential, or an
	// error (typically ErrUnauthorized / ErrVaultLocked).
	Resolve(ctx context.Context, bearer string) (Credential, error)
}

// VaultState is the process-wide record of the vault identity and (optionally) the
// unwrapped DEK. Populated either by /v1/vault/unlock (session mode) or by the
// auto_unlock boot hook in cmd/gvsvd. In api_keys mode it only holds the ID — DEK
// is unwrapped per-request from the api_keys row.
type VaultState struct {
	ID  uuid.UUID
	DEK []byte
}
