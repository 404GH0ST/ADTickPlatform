package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const defaultFlagFormatPrefix = "PLAYIT"

type flagClaims struct {
	Value       string
	OwnerTeamID int
	ChallengeID int
	IssuedTick  int
	ExpiresTick int
}

type flagCodec struct {
	secret []byte
	prefix string
}

func newFlagCodec(secret, prefix string) flagCodec {
	trimmedSecret := []byte(strings.TrimSpace(secret))
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		trimmedPrefix = defaultFlagFormatPrefix
	}
	return flagCodec{secret: trimmedSecret, prefix: trimmedPrefix}
}

func (c flagCodec) format() string {
	return c.prefix
}

func (c flagCodec) withPrefix(prefix string) flagCodec {
	return newFlagCodec(string(c.secret), prefix)
}

func (c flagCodec) Issue(ownerTeamID, challengeID, issuedTick, expiresTick int) string {
	payload := fmt.Sprintf("%d:%d:%d:%d", ownerTeamID, challengeID, issuedTick, expiresTick)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(encodedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return c.prefix + "{" + encodedPayload + "." + signature + "}"
}

func (c flagCodec) Parse(value string) (flagClaims, bool) {
	trimmed := strings.TrimSpace(value)
	expectedPrefix := c.prefix + "{"
	if !strings.HasPrefix(trimmed, expectedPrefix) || !strings.HasSuffix(trimmed, "}") {
		return flagClaims{}, false
	}
	body := trimmed[len(expectedPrefix) : len(trimmed)-1]
	parts := strings.Split(body, ".")
	if len(parts) != 2 {
		return flagClaims{}, false
	}

	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return flagClaims{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return flagClaims{}, false
	}
	fields := strings.Split(string(payload), ":")
	if len(fields) != 4 {
		return flagClaims{}, false
	}

	ownerTeamID, err := strconv.Atoi(fields[0])
	if err != nil {
		return flagClaims{}, false
	}
	challengeID, err := strconv.Atoi(fields[1])
	if err != nil {
		return flagClaims{}, false
	}
	issuedTick, err := strconv.Atoi(fields[2])
	if err != nil {
		return flagClaims{}, false
	}
	expiresTick, err := strconv.Atoi(fields[3])
	if err != nil {
		return flagClaims{}, false
	}

	return flagClaims{
		Value:       trimmed,
		OwnerTeamID: ownerTeamID,
		ChallengeID: challengeID,
		IssuedTick:  issuedTick,
		ExpiresTick: expiresTick,
	}, true
}
