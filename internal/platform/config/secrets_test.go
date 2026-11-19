package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSecretResolution(t *testing.T) {
	const primary = "ADPLATFORM_PRIMARY_TEST_SECRET"
	const fallback = "ADPLATFORM_FALLBACK_TEST_SECRET"
	t.Setenv(primary, "  test-primary-value  ")
	t.Setenv(fallback, "  test-fallback-value  ")
	if got := Secret(primary, fallback); got != "test-primary-value" {
		t.Fatalf("expected trimmed primary secret, got %q", got)
	}
	t.Setenv(primary, "  ")
	if got := Secret(primary, fallback); got != "test-fallback-value" {
		t.Fatalf("expected trimmed fallback secret, got %q", got)
	}
}

func TestRequiredSecretRejectsInvalidConfiguration(t *testing.T) {
	const key = "ADPLATFORM_INVALID_TEST_SECRET"
	const childKey = "ADPLATFORM_VALIDATE_TEST_SECRET"
	if os.Getenv(childKey) == "1" {
		Secret(key, "")
		return
	}

	for _, tt := range []struct {
		name, value, message string
	}{
		{"missing", "   ", "must be set"},
		{"known placeholder", "dev-admin-token", "must not use the example or development value"},
		{"placeholder prefix", "replace-with-test-value", "must not use the example or development value"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestRequiredSecretRejectsInvalidConfiguration$")
			cmd.Env = append(os.Environ(), childKey+"=1", key+"="+tt.value)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), key+" "+tt.message) {
				t.Fatalf("expected startup to fail with %q, got %v: %s", tt.message, err, output)
			}
		})
	}
}
