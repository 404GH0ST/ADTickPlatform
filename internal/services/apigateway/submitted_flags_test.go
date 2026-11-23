package apigateway

import (
	"strings"
	"testing"
)

func TestValidateSubmittedFlagsRejectsNUL(t *testing.T) {
	if err := ValidateSubmittedFlags([]string{"valid-looking", "x\x00y"}); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("a NUL byte must reject the batch before history is written: %v", err)
	}
	if err := ValidateSubmittedFlags([]string{"PLAYIT{value}"}); err != nil {
		t.Fatalf("ordinary flag rejected: %v", err)
	}
}
