// Package auth handles GitHub OAuth (web flow), session JWTs for the UI, and
// long-lived bearer tokens for the CLI.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ── Sessions (UI) ─────────────────────────────────────────────────────────

type Session struct {
	UserID int64 `json:"uid"`
	Exp    int64 `json:"exp"`
}

type SessionSigner struct {
	secret []byte
}

func NewSessionSigner(secret []byte) *SessionSigner {
	return &SessionSigner{secret: secret}
}

func (s *SessionSigner) Sign(userID int64, ttl time.Duration) (string, error) {
	payload := Session{UserID: userID, Exp: time.Now().Add(ttl).Unix()}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig, nil
}

func (s *SessionSigner) Verify(token string) (Session, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return Session{}, errors.New("invalid session")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return Session{}, errors.New("bad signature")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Session{}, err
	}
	var sess Session
	if err := json.Unmarshal(body, &sess); err != nil {
		return Session{}, err
	}
	if time.Now().Unix() > sess.Exp {
		return Session{}, errors.New("session expired")
	}
	return sess, nil
}

// ── CLI bearer tokens ─────────────────────────────────────────────────────

// NewAPIToken returns a printable token (with "mgt_" prefix) and the SHA-256
// hash to store in the database.
func NewAPIToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = "mgt_" + base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

func HashAPIToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// ── CLI device-code-style flow ────────────────────────────────────────────

// DeviceFlow tracks pending CLI logins. The CLI hits /auth/cli/start to
// receive a "user_code" and a verification URL the user opens in a browser;
// after GitHub OAuth completes, the server links the issued API token to the
// user_code, and the CLI's /auth/cli/poll call drains it.
//
// Stored in-memory with a TTL — fine for self-host (single process).
type DeviceFlow struct {
	codes map[string]*pendingDevice
}

type pendingDevice struct {
	userCode string
	state    string
	exp      time.Time
	token    string // populated after OAuth completes
	userID   int64
	mu       chan struct{} // closed when token ready
}

func NewDeviceFlow() *DeviceFlow {
	return &DeviceFlow{codes: map[string]*pendingDevice{}}
}

func (f *DeviceFlow) Start() (userCode, state string, err error) {
	uc := mustHex(4)
	st := mustHex(16)
	f.codes[st] = &pendingDevice{
		userCode: uc,
		state:    st,
		exp:      time.Now().Add(10 * time.Minute),
		mu:       make(chan struct{}),
	}
	return uc, st, nil
}

func (f *DeviceFlow) Complete(state string, userID int64, token string) error {
	pd, ok := f.codes[state]
	if !ok {
		return errors.New("unknown state")
	}
	pd.userID = userID
	pd.token = token
	close(pd.mu)
	return nil
}

func (f *DeviceFlow) Poll(ctx context.Context, state string) (string, int64, error) {
	pd, ok := f.codes[state]
	if !ok {
		return "", 0, errors.New("unknown state")
	}
	select {
	case <-pd.mu:
		delete(f.codes, state)
		return pd.token, pd.userID, nil
	case <-ctx.Done():
		return "", 0, ctx.Err()
	case <-time.After(time.Until(pd.exp)):
		delete(f.codes, state)
		return "", 0, errors.New("device flow expired")
	}
}

// UserCode returns the human-friendly code for a pending state, used to render
// it on the OAuth confirmation page.
func (f *DeviceFlow) UserCode(state string) (string, bool) {
	pd, ok := f.codes[state]
	if !ok {
		return "", false
	}
	return pd.userCode, true
}

func mustHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("rand: %v", err))
	}
	return hex.EncodeToString(b)
}
