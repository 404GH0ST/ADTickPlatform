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

// Issue returns the unlock proof for a team/challenge at the given epoch.
// Epoch must be >= 1; values below 1 are treated as 1.
//
// Epoch 1 uses the legacy MAC payload (challenge:team) so existing mid-event
// deployments keep working until organizers intentionally rotate. Epoch 2+
// includes the epoch in the MAC so old stolen proofs stop verifying.
func Issue(secret string, teamID, challengeID, epoch int) string {
	payload := payload(teamID, challengeID, normalizeEpoch(epoch))
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%d.%d.%s", prefix, challengeID, teamID, signature)
}

// Verify checks proof against the current epoch for the team/challenge.
func Verify(secret string, teamID, challengeID, epoch int, proof string) bool {
	trimmed := strings.TrimSpace(proof)
	if trimmed == "" || strings.TrimSpace(secret) == "" {
		return false
	}
	expected := Issue(secret, teamID, challengeID, epoch)
	if len(trimmed) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(trimmed), []byte(expected)) == 1
}

func normalizeEpoch(epoch int) int {
	if epoch < 1 {
		return 1
	}
	return epoch
}

func payload(teamID, challengeID, epoch int) string {
	// Epoch 1 keeps pre-rotation MAC input so upgrading the platform without
	// bumping epoch does not mass-invalidate every team's unlock proof.
	if epoch <= 1 {
		return fmt.Sprintf("%d:%d", challengeID, teamID)
	}
	return fmt.Sprintf("%d:%d:%d", challengeID, teamID, epoch)
}
