package unlockproof

import "testing"

func TestIssueAndVerify(t *testing.T) {
	secret := "dev-unlock-secret"
	proof := Issue(secret, 101, 2)

	if !Verify(secret, 101, 2, proof) {
		t.Fatal("expected issued proof to verify")
	}
	if Verify(secret, 102, 2, proof) {
		t.Fatal("expected proof verification to fail for different team")
	}
	if Verify(secret, 101, 3, proof) {
		t.Fatal("expected proof verification to fail for different challenge")
	}
	if Verify("other-secret", 101, 2, proof) {
		t.Fatal("expected proof verification to fail for different secret")
	}
}
