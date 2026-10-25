package config

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"

	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// The browser-extension pairing token lives in its own 0600 file, NOT in
// settings.json: settings.json is 0644 and returned verbatim by GET /settings,
// which would hand the secret to anything that can read either.
const (
	extTokenFile   = "extension_token"
	extTokenPrefix = "rum_"
	extTokenBytes  = 32
	// extTokenMinLen rejects a truncated or hand-edited file so it can never
	// become a weak token.
	extTokenMinLen = len(extTokenPrefix) + 2*extTokenBytes
)

var extTokenMu sync.Mutex

func newExtensionToken() (string, error) {
	b := make([]byte, extTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return extTokenPrefix + hex.EncodeToString(b), nil
}

func writeExtensionToken(tok string) error {
	path := filesystem.CreateMetadataFile(extTokenFile)
	return filesystem.AtomicWriteFile(path, []byte(tok+"\n"), 0o600)
}

func readExtensionToken() string {
	data, err := filesystem.ReadMetadataFile(extTokenFile)
	if err != nil {
		return ""
	}
	tok := strings.TrimSpace(string(data))
	if len(tok) < extTokenMinLen || !strings.HasPrefix(tok, extTokenPrefix) {
		return ""
	}
	return tok
}

// ExtensionToken returns the pairing token, creating it on first use.
func ExtensionToken() (string, error) {
	extTokenMu.Lock()
	defer extTokenMu.Unlock()
	if tok := readExtensionToken(); tok != "" {
		return tok, nil
	}
	tok, err := newExtensionToken()
	if err != nil {
		return "", err
	}
	if err := writeExtensionToken(tok); err != nil {
		return "", err
	}
	return tok, nil
}

// RegenerateExtensionToken replaces the token; the old one stops working at once.
func RegenerateExtensionToken() (string, error) {
	extTokenMu.Lock()
	defer extTokenMu.Unlock()
	tok, err := newExtensionToken()
	if err != nil {
		return "", err
	}
	if err := writeExtensionToken(tok); err != nil {
		return "", err
	}
	return tok, nil
}

// VerifyExtensionToken reports whether got is the current token. Both sides are
// hashed first so the comparison is constant-time regardless of length. It never
// creates a token: until the owner has opened Settings (ExtensionToken) nothing
// can be paired, so every guess fails.
func VerifyExtensionToken(got string) bool {
	want := readExtensionToken()
	if got == "" || want == "" {
		return false
	}
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
