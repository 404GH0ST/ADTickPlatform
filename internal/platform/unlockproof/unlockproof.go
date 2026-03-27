package unlockproof

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

const prefix = "ADU1"

func Issue(secret string, teamID, challengeID int) string {
	payload := payload(teamID, challengeID)
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%d.%d.%s", prefix, challengeID, teamID, signature)
}

func Verify(secret string, teamID, challengeID int, proof string) bool {
	trimmed := strings.TrimSpace(proof)
	if trimmed == "" || strings.TrimSpace(secret) == "" {
		return false
	}
	expected := Issue(secret, teamID, challengeID)
	if len(trimmed) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(trimmed), []byte(expected)) == 1
}

func payload(teamID, challengeID int) string {
	return fmt.Sprintf("%d:%d", challengeID, teamID)
}
