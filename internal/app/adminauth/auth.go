package adminauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	CookieName            = "codex_remote_admin_session"
	DefaultSetupTokenTTL  = 20 * time.Minute
	defaultSessionKeySize = 32
)

type Scope string

const (
	ScopeSetup Scope = "setup"
	ScopeAdmin Scope = "admin"
)

var (
	ErrExpired        = errors.New("auth expired")
	ErrInvalidToken   = errors.New("invalid setup token")
	ErrInvalidSession = errors.New("invalid session")
	ErrMissingToken   = errors.New("missing setup token")
	ErrSetupDisabled  = errors.New("setup access disabled")
)

type ManagerOptions struct {
	Now        func() time.Time
	SessionKey []byte
}

type Manager struct {
	now func() time.Time

	sessionKey []byte

	mu sync.RWMutex

	setupEnabled bool
	setupHash    [32]byte
	setupExpiry  time.Time
}

type Session struct {
	Scope     Scope     `json:"scope"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type sessionPayload struct {
	Scope Scope `json:"scope"`
	Exp   int64 `json:"exp"`
}

func NewManager(opts ManagerOptions) (*Manager, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	sessionKey := append([]byte(nil), opts.SessionKey...)
	if len(sessionKey) == 0 {
		sessionKey = make([]byte, defaultSessionKeySize)
		if _, err := rand.Read(sessionKey); err != nil {
			return nil, err
		}
	}
	return &Manager{
		now:        now,
		sessionKey: sessionKey,
	}, nil
}

func (m *Manager) EnableSetupToken(ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = DefaultSetupTokenTTL
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	expiresAt := m.now().Add(ttl).UTC()

	m.mu.Lock()
	m.setupEnabled = true
	m.setupHash = hash
	m.setupExpiry = expiresAt
	m.mu.Unlock()

	return token, expiresAt, nil
}

func (m *Manager) DisableSetupToken() {
	m.mu.Lock()
	m.setupEnabled = false
	m.setupHash = [32]byte{}
	m.setupExpiry = time.Time{}
	m.mu.Unlock()
}

func (m *Manager) SetupStatus() (bool, time.Time) {
	m.mu.RLock()
	enabled := m.setupEnabled
	expiresAt := m.setupExpiry
	m.mu.RUnlock()

	if !enabled {
		return false, time.Time{}
	}
	if !expiresAt.After(m.now()) {
		return false, time.Time{}
	}
	return true, expiresAt
}

func (m *Manager) ValidateSetupToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrMissingToken
	}

	m.mu.RLock()
	enabled := m.setupEnabled
	expected := m.setupHash
	expiresAt := m.setupExpiry
	m.mu.RUnlock()

	if !enabled {
		return ErrSetupDisabled
	}
	if !expiresAt.After(m.now()) {
		return ErrExpired
	}

	actual := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		return ErrInvalidToken
	}
	return nil
}

func (m *Manager) ExchangeSetupToken(token string) (string, time.Time, error) {
	if err := m.ValidateSetupToken(token); err != nil {
		return "", time.Time{}, err
	}
	_, expiresAt := m.SetupStatus()
	value, err := m.NewSession(ScopeSetup, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return value, expiresAt, nil
}

func (m *Manager) NewSession(scope Scope, expiresAt time.Time) (string, error) {
	expiresAt = expiresAt.UTC()
	if scope == "" {
		return "", ErrInvalidSession
	}
	if !expiresAt.After(m.now()) {
		return "", ErrExpired
	}

	payload, err := json.Marshal(sessionPayload{
		Scope: scope,
		Exp:   expiresAt.Unix(),
	})
	if err != nil {
		return "", err
	}

	sig := m.sign(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (m *Manager) ParseSession(value string) (Session, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 2 {
		return Session{}, ErrInvalidSession
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	expected := m.sign(payload)
	if !hmac.Equal(sig, expected) {
		return Session{}, ErrInvalidSession
	}

	var decoded sessionPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Session{}, ErrInvalidSession
	}
	expiresAt := time.Unix(decoded.Exp, 0).UTC()
	if !expiresAt.After(m.now()) {
		return Session{}, ErrExpired
	}
	if decoded.Scope == "" {
		return Session{}, ErrInvalidSession
	}
	return Session{
		Scope:     decoded.Scope,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *Manager) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, m.sessionKey)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func SessionCookie(value string, expiresAt time.Time) *http.Cookie {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
		MaxAge:   maxAge,
	}
}

func ExpiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	}
}

func IsLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
