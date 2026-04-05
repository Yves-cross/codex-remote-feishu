package install

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunMainHelpReturnsNil(t *testing.T) {
	var stdout bytes.Buffer
	err := RunMain([]string{"-h"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("RunMain(-h): %v", err)
	}
	if !strings.Contains(stdout.String(), "-binary") {
		t.Fatalf("help output missing -binary flag: %q", stdout.String())
	}
}
