package apigateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newCheckerToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate checker token: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
