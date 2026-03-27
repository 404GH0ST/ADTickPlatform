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

type flagClaims struct {
	Value       string
	OwnerTeamID int
	ChallengeID int
	IssuedTick  int
	ExpiresTick int
}

type flagCodec struct {
	secret []byte
}

func newFlagCodec(secret string) flagCodec {
	return flagCodec{secret: []byte(strings.TrimSpace(secret))}
}

func (c flagCodec) Issue(ownerTeamID, challengeID, issuedTick, expiresTick int) string {
	payload := fmt.Sprintf("%d:%d:%d:%d", ownerTeamID, challengeID, issuedTick, expiresTick)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(encodedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return "FLAGv1." + encodedPayload + "." + signature
}

func (c flagCodec) Parse(value string) (flagClaims, bool) {
	trimmed := strings.TrimSpace(value)
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 || parts[0] != "FLAGv1" {
		return flagClaims{}, false
	}

	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(parts[1]))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return flagClaims{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
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
