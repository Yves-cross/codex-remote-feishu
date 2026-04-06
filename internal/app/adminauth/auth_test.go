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
		Now:        func() time.Time { return now },
		SessionKey: []byte("01234567890123456789012345678901"),
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
	if !sessionExpiresAt.Equal(expiresAt) {
		t.Fatalf("session expiry = %v, want %v", sessionExpiresAt, expiresAt)
	}

	session, err := manager.ParseSession(value)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if session.Scope != ScopeSetup {
		t.Fatalf("session scope = %q, want %q", session.Scope, ScopeSetup)
	}
	if !session.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("session expiry = %v, want %v", session.ExpiresAt, expiresAt)
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
