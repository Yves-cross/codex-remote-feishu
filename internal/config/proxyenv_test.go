package config

import (
	"os"
	"testing"
)

func TestCaptureAndClearProxyEnv(t *testing.T) {
	t.Setenv("http_proxy", "http://127.0.0.1:9999")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9998")

	restored := CaptureAndClearProxyEnv()
	if len(restored) < 2 {
		t.Fatalf("unexpected restored size: %d", len(restored))
	}
	if _, ok := os.LookupEnv("http_proxy"); ok {
		t.Fatal("expected http_proxy to be cleared")
	}
	if _, ok := os.LookupEnv("HTTPS_PROXY"); ok {
		t.Fatal("expected HTTPS_PROXY to be cleared")
	}
}

func TestFilterEnvWithoutProxy(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"http_proxy=http://127.0.0.1:9999",
		"HTTPS_PROXY=http://127.0.0.1:9998",
		"HOME=/tmp",
	}
	filtered := FilterEnvWithoutProxy(input)
	if len(filtered) != 2 {
		t.Fatalf("unexpected filtered size: %d", len(filtered))
	}
	for _, entry := range filtered {
		if entry == "http_proxy=http://127.0.0.1:9999" || entry == "HTTPS_PROXY=http://127.0.0.1:9998" {
			t.Fatalf("proxy entry leaked: %s", entry)
		}
	}
}
