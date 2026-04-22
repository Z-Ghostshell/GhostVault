// Package authdebug formats bearer/session tokens for diagnostic logs without always printing secrets.
package authdebug

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Fingerprint is the first 8 bytes of SHA-256(token), hex-encoded (16 chars). Compare client vs server logs.
func Fingerprint(token string) string {
	t := strings.TrimSpace(token)
	if t == "" {
		return "<empty>"
	}
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:8])
}

// ForLog returns token text for logs: masked unless revealFull is true.
func ForLog(token string, revealFull bool) string {
	t := strings.TrimSpace(token)
	if t == "" {
		return "<empty>"
	}
	if revealFull {
		return t
	}
	if len(t) <= 8 {
		return fmt.Sprintf("<len=%d>", len(t))
	}
	return fmt.Sprintf("%s…%s(len=%d)", t[:4], t[len(t)-4:], len(t))
}
