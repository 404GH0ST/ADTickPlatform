package config

import (
	"testing"
	"time"
)

func TestEnvironmentReaders(t *testing.T) {
	const key = "ADPLATFORM_CONFIG_TEST"

	for _, value := range []string{"", "invalid"} {
		t.Setenv(key, value)
		if got := Int(key, 7); got != 7 {
			t.Errorf("Int with %q = %d, want fallback", value, got)
		}
		if got := Bool(key, true); !got {
			t.Errorf("Bool with %q = %t, want fallback", value, got)
		}
		if got := Duration(key, time.Minute); got != time.Minute {
			t.Errorf("Duration with %q = %v, want fallback", value, got)
		}
		if got := Float(key, 1.5); got != 1.5 {
			t.Errorf("Float with %q = %v, want fallback", value, got)
		}
	}

	t.Setenv(key, "")
	if got := String(key, "default"); got != "default" {
		t.Errorf("String with empty value = %q, want fallback", got)
	}
	t.Setenv(key, "-12")
	if got := Int(key, 7); got != -12 {
		t.Errorf("Int = %d, want -12", got)
	}
	t.Setenv(key, "false")
	if got := Bool(key, true); got {
		t.Errorf("Bool = %t, want false", got)
	}
	t.Setenv(key, "2m30s")
	if got := Duration(key, 0); got != 150*time.Second {
		t.Errorf("Duration = %v, want 150s", got)
	}
	t.Setenv(key, " 0.25 ")
	if got := Float(key, 0); got != 0.25 {
		t.Errorf("Float = %v, want 0.25", got)
	}
	if got := String(key, ""); got != " 0.25 " {
		t.Errorf("String = %q, want original value", got)
	}
}
