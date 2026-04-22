package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/z-ghostshell/ghostvault/internal/store"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrVaultLocked = errors.New("vault locked or session expired")

// ErrAuthBackend wraps a store or infrastructure failure while resolving credentials.
// Callers should return HTTP 503/500 with a redacted body and log the wrapped error.
var ErrAuthBackend = errors.New("auth backend error")

// Session is the historical in-process view of an unlocked credential. It implements
// Credential and is also passed down into core.Engine for DEK access during ingest/retrieve.
type Session struct {
	VaultID   uuid.UUID
	DEK       []byte // nil when encryption off
	ExpiresAt time.Time
	MaxUntil  time.Time
	LastSeen  time.Time
	Idle      time.Duration
	tokenHash []byte
}

// --- Credential implementation ------------------------------------------------

func (s *Session) vaultID() uuid.UUID { return s.VaultID }

// Credential getters. Kept as methods for the interface; public fields remain for
// the existing engine call sites that read s.DEK / s.VaultID directly.
func (s *Session) TokenHash() []byte { return s.tokenHash }
func (s *Session) Scopes() []string  { return nil }

// Interface assertion (via wrapper methods that satisfy Credential naming).
type sessionCredential struct{ *Session }

func (c sessionCredential) VaultID() uuid.UUID { return c.Session.VaultID }
func (c sessionCredential) DEK() []byte        { return c.Session.DEK }
func (c sessionCredential) Scopes() []string   { return c.Session.Scopes() }
func (c sessionCredential) TokenHash() []byte  { return c.Session.TokenHash() }

// --- Manager ------------------------------------------------------------------

// Manager is the DB-backed session resolver shared by auth_mode=session and
// auth_mode=auto_unlock. It writes through a Postgres `sessions` table so rows
// survive restart; whether they are also valid after restart depends on whether
// VaultState.DEK can be rehydrated (auto_unlock does so, plain session does not —
// gvsvd purges the table on boot in that case).
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	idle     time.Duration
	max      time.Duration

	store *store.Pool
	state *VaultState // shared pointer; DEK may be nil when encryption is off or before unlock
}

// NewManager preserves the old constructor for callers that haven't been wired to
// the store + VaultState yet (tests).
func NewManager(idle, max time.Duration) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		idle:     idle,
		max:      max,
	}
}

// NewManagerWithStore is the production constructor. store may be nil for in-memory
// tests; state must be non-nil so Touch can attach the current DEK.
func NewManagerWithStore(idle, max time.Duration, st *store.Pool, state *VaultState) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		idle:     idle,
		max:      max,
		store:    st,
		state:    state,
	}
}

// SetVaultState lets gvsvd startup swap in the final VaultState (after auto-unlock
// or after the first /v1/vault/unlock mints a DEK).
func (m *Manager) SetVaultState(state *VaultState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

func hashToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

func (m *Manager) CreateSession(ctx context.Context, vault uuid.UUID, dek []byte, now time.Time) (token string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	th := hashToken(tok)
	expiresAt := now.Add(m.idle)
	maxUntil := now.Add(m.max)

	if m.store != nil {
		if err := m.store.InsertSession(ctx, th, vault, now, expiresAt, maxUntil); err != nil {
			return "", err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Keep the DEK on the in-memory copy for cheap reads; on cache miss after
	// restart we re-fill from m.state.DEK.
	m.sessions[tok] = &Session{
		VaultID:   vault,
		DEK:       dek,
		ExpiresAt: expiresAt,
		MaxUntil:  maxUntil,
		LastSeen:  now,
		Idle:      m.idle,
		tokenHash: th,
	}
	// If a VaultState pointer exists and was empty (encryption=on, before unlock),
	// fill it now so subsequent Resolves from cache misses can use the DEK.
	if m.state != nil && len(m.state.DEK) == 0 && len(dek) > 0 {
		m.state.DEK = dek
		m.state.ID = vault
	}
	return tok, nil
}

// Touch validates a token and extends its TTL, writing through to the DB.
func (m *Manager) Touch(ctx context.Context, token string, now time.Time) (*Session, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	th := hashToken(token)

	m.mu.Lock()
	s, ok := m.sessions[token]
	m.mu.Unlock()

	if !ok {
		// Try DB (persistent session across restart, e.g. auto_unlock mode).
		if m.store == nil {
			return nil, ErrUnauthorized
		}
		row, err := m.store.GetSessionByHash(ctx, th)
		if err != nil {
			return nil, ErrUnauthorized
		}
		if now.After(row.MaxUntil) || now.Sub(row.LastSeen) > m.idle {
			_ = m.store.DeleteSession(ctx, th)
			return nil, ErrVaultLocked
		}
		// Rehydrate the in-memory session. DEK comes from VaultState — if it's not
		// populated (session mode post-restart without auto_unlock), treat as locked.
		var dek []byte
		if m.state != nil {
			dek = m.state.DEK
		}
		s = &Session{
			VaultID:   row.VaultID,
			DEK:       dek,
			ExpiresAt: row.ExpiresAt,
			MaxUntil:  row.MaxUntil,
			LastSeen:  row.LastSeen,
			Idle:      m.idle,
			tokenHash: append([]byte(nil), th...),
		}
		m.mu.Lock()
		m.sessions[token] = s
		m.mu.Unlock()
	}

	m.mu.Lock()
	if now.After(s.MaxUntil) {
		delete(m.sessions, token)
		m.mu.Unlock()
		if m.store != nil {
			_ = m.store.DeleteSession(ctx, th)
		}
		return nil, ErrVaultLocked
	}
	if now.Sub(s.LastSeen) > s.Idle {
		delete(m.sessions, token)
		m.mu.Unlock()
		if m.store != nil {
			_ = m.store.DeleteSession(ctx, th)
		}
		return nil, ErrVaultLocked
	}
	s.LastSeen = now
	s.ExpiresAt = now.Add(s.Idle)
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.TouchSession(ctx, th, s.LastSeen, s.ExpiresAt)
	}
	return s, nil
}

func (m *Manager) Invalidate(ctx context.Context, token string) {
	th := hashToken(token)
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
	if m.store != nil {
		_ = m.store.DeleteSession(ctx, th)
	}
}

// InvalidateAll clears the in-memory cache only. Callers that want to wipe
// persisted sessions should use store.PurgeSessionsForVault.
func (m *Manager) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*Session)
}

// Resolve implements CredentialResolver for the session / auto_unlock modes.
func (m *Manager) Resolve(ctx context.Context, bearer string) (Credential, error) {
	s, err := m.Touch(ctx, bearer, time.Now())
	if err != nil {
		return nil, err
	}
	return sessionCredential{Session: s}, nil
}

// HashActionToken preserves the pre-refactor helper for action_tokens (plaintext vaults).
func HashActionToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func NewRandomToken() (raw string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
