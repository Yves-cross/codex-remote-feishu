package adminauth

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetupTokenExchangeProducesValidSetupSession(t *testing.T) {
	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	manager, err := NewManager(ManagerOptions{
		Now:             func() time.Time { return now },
		SessionKey:      []byte("01234567890123456789012345678901"),
		SetupSessionTTL: 2 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	token, expiresAt, err := manager.EnableSetupToken(15 * time.Minute)
	if err != nil {
		t.Fatalf("EnableSetupToken: %v", err)
	}
	value, sessionExpiresAt, err := manager.ExchangeSetupToken(token)
	if err != nil {
		t.Fatalf("ExchangeSetupToken: %v", err)
	}
	if !expiresAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("token expiry = %v, want %v", expiresAt, now.Add(15*time.Minute))
	}
	if !sessionExpiresAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("session expiry = %v, want %v", sessionExpiresAt, now.Add(2*time.Hour))
	}

	session, err := manager.ParseSession(value)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if session.Scope != ScopeSetup {
		t.Fatalf("session scope = %q, want %q", session.Scope, ScopeSetup)
	}
	if !session.ExpiresAt.Equal(sessionExpiresAt) {
		t.Fatalf("session expiry = %v, want %v", session.ExpiresAt, sessionExpiresAt)
	}

	now = now.Add(30 * time.Minute)
	if err := manager.ValidateSetupToken(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("ValidateSetupToken(expired) err = %v, want %v", err, ErrExpired)
	}
	if _, err := manager.ParseSession(value); err != nil {
		t.Fatalf("ParseSession(session still valid): %v", err)
	}
}

func TestSetupTokenValidationRejectsWrongAndExpiredTokens(t *testing.T) {
	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	manager, err := NewManager(ManagerOptions{
		Now:        func() time.Time { return now },
		SessionKey: []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	token, _, err := manager.EnableSetupToken(time.Minute)
	if err != nil {
		t.Fatalf("EnableSetupToken: %v", err)
	}
	if err := manager.ValidateSetupToken("wrong-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ValidateSetupToken(wrong) err = %v, want %v", err, ErrInvalidToken)
	}

	now = now.Add(2 * time.Minute)
	if err := manager.ValidateSetupToken(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("ValidateSetupToken(expired) err = %v, want %v", err, ErrExpired)
	}
}

func TestDisableSetupTokenRevokesSetupSessions(t *testing.T) {
	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	manager, err := NewManager(ManagerOptions{
		Now:             func() time.Time { return now },
		SessionKey:      []byte("01234567890123456789012345678901"),
		SetupSessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	token, _, err := manager.EnableSetupToken(10 * time.Minute)
	if err != nil {
		t.Fatalf("EnableSetupToken: %v", err)
	}
	value, _, err := manager.ExchangeSetupToken(token)
	if err != nil {
		t.Fatalf("ExchangeSetupToken: %v", err)
	}

	manager.DisableSetupToken()
	if _, err := manager.ParseSession(value); !errors.Is(err, ErrSetupDisabled) {
		t.Fatalf("ParseSession(disabled) err = %v, want %v", err, ErrSetupDisabled)
	}
}

func TestParseSessionRejectsTamperedCookie(t *testing.T) {
	now := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	manager, err := NewManager(ManagerOptions{
		Now:        func() time.Time { return now },
		SessionKey: []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	value, err := manager.NewSession(ScopeAdmin, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	value += "tampered"
	if _, err := manager.ParseSession(value); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("ParseSession(tampered) err = %v, want %v", err, ErrInvalidSession)
	}
}

func TestIsLoopbackRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if !IsLoopbackRequest(req) {
		t.Fatal("expected loopback request")
	}

	req = httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "192.0.2.10:12345"
	if IsLoopbackRequest(req) {
		t.Fatal("expected non-loopback request")
	}
}
