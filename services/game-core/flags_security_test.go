package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// signedFlag builds a correctly signed flag around an arbitrary payload, so the
// parser's payload validation can be exercised past the signature check.
func signedFlag(secret, prefix, payload string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(encoded))
	return prefix + "{" + encoded + "." + hex.EncodeToString(mac.Sum(nil)) + "}"
}

func TestFlagCodecRejectsMalformedSignedPayloads(t *testing.T) {
	codec := newFlagCodec("flag-secret", "CTF")
	for _, payload := range []string{
		"1:2:3",           // too few fields
		"1:2:3:4:nonce:x", // too many fields
		"x:2:3:4:nonce",   // non-numeric owner
		"1:x:3:4:nonce",   // non-numeric challenge
		"1:2:x:4:nonce",   // non-numeric issued tick
		"1:2:3:x:nonce",   // non-numeric expiry
		"1:2:3:4:",        // empty nonce
		"",                // empty
	} {
		flag := signedFlag("flag-secret", "CTF", payload)
		if claims, ok := codec.Parse(flag); ok {
			t.Fatalf("payload %q must be rejected even when signed, got %+v", payload, claims)
		}
	}
}

func TestFlagCodecAcceptsLegacyFlagForAnActiveMatch(t *testing.T) {
	codec := newFlagCodec("flag-secret", "CTF")
	legacy := codec.issueWithNonce(101, 2, 5, 9, "")
	claims, ok := codec.Parse(legacy)
	if !ok || claims.OwnerTeamID != 101 || claims.ChallengeID != 2 || claims.Nonce != "" {
		t.Fatalf("legacy issued flag should remain valid until its tick expires: %+v, %v", claims, ok)
	}
}

func TestFlagCodecRejectsStructuralForgeries(t *testing.T) {
	codec := newFlagCodec("flag-secret", "CTF")
	valid := codec.Issue(101, 2, 5, 9)
	body := strings.TrimSuffix(strings.TrimPrefix(valid, "CTF{"), "}")
	payload, signature, _ := strings.Cut(body, ".")
	for name, candidate := range map[string]string{
		"missing suffix":      strings.TrimSuffix(valid, "}"),
		"extra segment":       "CTF{" + payload + "." + signature + ".x}",
		"no separator":        "CTF{" + payload + signature + "}",
		"uppercase signature": "CTF{" + payload + "." + strings.ToUpper(signature) + "}",
		"truncated signature": "CTF{" + payload + "." + signature[:len(signature)-2] + "}",
		"padded payload":      "CTF{" + payload + "==." + signature + "}",
		"other key":           newFlagCodec("other-secret", "CTF").Issue(101, 2, 5, 9),
		"other prefix":        "PLAYIT{" + body + "}",
		"lowercase prefix":    "ctf{" + body + "}",
		"undecodable payload": signedFlag("flag-secret", "CTF", "")[:4] + "!!!." + signature + "}",
	} {
		if _, ok := codec.Parse(candidate); ok {
			t.Errorf("%s: forged flag %q was accepted", name, candidate)
		}
	}
	if _, ok := codec.Parse("  " + valid + "\n"); !ok {
		t.Fatal("surrounding whitespace on a genuine flag must still parse")
	}
}

func TestFlagCodecRejectsUndecodableSignedPayload(t *testing.T) {
	codec := newFlagCodec("flag-secret", "CTF")
	// "!!!" is not base64url, but the MAC is computed over the raw text, so the
	// signature check passes and the decode step must be what rejects it.
	mac := hmac.New(sha256.New, []byte("flag-secret"))
	mac.Write([]byte("!!!"))
	flag := "CTF{!!!." + hex.EncodeToString(mac.Sum(nil)) + "}"
	if _, ok := codec.Parse(flag); ok {
		t.Fatal("an undecodable payload must be rejected")
	}
}

// FuzzFlagCodecParseIsCanonical checks the anti-replay property duplicate
// detection relies on: anything Parse accepts must be exactly the string
// issueWithNonce produces for those claims and nonce.
func FuzzFlagCodecParseIsCanonical(f *testing.F) {
	codec := newFlagCodec("fuzz-secret", "CTF")
	f.Add(codec.Issue(101, 2, 5, 9))
	f.Add(codec.Issue(0, 0, 0, 0))
	f.Add(codec.Issue(-1, 2147483647, 3, -9))
	f.Add("CTF{}")
	f.Add("CTF{a.b}")
	f.Fuzz(func(t *testing.T, candidate string) {
		claims, ok := codec.Parse(candidate)
		if !ok {
			return
		}
		canonical := codec.issueWithNonce(claims.OwnerTeamID, claims.ChallengeID, claims.IssuedTick, claims.ExpiresTick, claims.Nonce)
		if strings.TrimSpace(candidate) != canonical {
			t.Fatalf("accepted non-canonical flag %q; canonical form is %q", candidate, canonical)
		}
		if claims.Value != canonical {
			t.Fatalf("claims value %q differs from canonical %q", claims.Value, canonical)
		}
	})
}
