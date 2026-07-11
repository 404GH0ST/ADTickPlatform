package unlockproof

import "testing"

func TestIssueAndVerify(t *testing.T) {
	secret := "dev-unlock-secret"
	proof := Issue(secret, 101, 2, 1)

	if !Verify(secret, 101, 2, 1, proof) {
		t.Fatal("expected issued proof to verify")
	}
	if Verify(secret, 102, 2, 1, proof) {
		t.Fatal("expected proof verification to fail for different team")
	}
	if Verify(secret, 101, 3, 1, proof) {
		t.Fatal("expected proof verification to fail for different challenge")
	}
	if Verify("other-secret", 101, 2, 1, proof) {
		t.Fatal("expected proof verification to fail for different secret")
	}
}

func TestEpochRotationInvalidatesOldProofs(t *testing.T) {
	secret := "dev-unlock-secret"
	epoch1 := Issue(secret, 101, 2, 1)
	epoch2 := Issue(secret, 101, 2, 2)

	if epoch1 == epoch2 {
		t.Fatal("expected epoch bump to change proof material")
	}
	if !Verify(secret, 101, 2, 1, epoch1) {
		t.Fatal("epoch 1 proof should verify at epoch 1")
	}
	if Verify(secret, 101, 2, 2, epoch1) {
		t.Fatal("epoch 1 proof must not verify after rotation to epoch 2")
	}
	if !Verify(secret, 101, 2, 2, epoch2) {
		t.Fatal("epoch 2 proof should verify at epoch 2")
	}
	if Verify(secret, 101, 2, 1, epoch2) {
		t.Fatal("epoch 2 proof must not verify at epoch 1")
	}
}

func TestEpochOneMatchesLegacyPayloadShape(t *testing.T) {
	// Guard: epoch 1 MAC input stays "challenge:team" (no epoch suffix).
	secret := "dev-unlock-secret"
	a := Issue(secret, 7, 3, 1)
	b := Issue(secret, 7, 3, 0) // normalized to 1
	if a != b {
		t.Fatalf("epoch 0 should normalize to epoch 1 proof, got %q vs %q", a, b)
	}
	// Distinct from epoch 2.
	if Issue(secret, 7, 3, 2) == a {
		t.Fatal("epoch 2 must differ from epoch 1")
	}
}
