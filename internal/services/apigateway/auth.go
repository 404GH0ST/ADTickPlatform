package apigateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidTeamToken = errors.New("invalid team token")

type teamTokenClaims struct {
	TeamID           int    `json:"team_id"`
	PlayerID         int    `json:"player_id"`
	TeamName         string `json:"team_name"`
	TeamContactEmail string `json:"team_contact_email"`
	DisplayName      string `json:"display_name"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	SessionVersion   int    `json:"session_version"`
	IssuedAt         int64  `json:"iat"`
	ExpiresAt        int64  `json:"exp"`
}

func issueTeamJWT(secret string, player authenticatedPlayer, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("team token secret is empty")
	}
	player = canonicalSessionPlayer(player)

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	sessionVersion := player.SessionVersion
	if sessionVersion < 1 {
		sessionVersion = 1
	}
	claimsJSON, err := json.Marshal(teamTokenClaims{
		TeamID:           player.TeamID,
		PlayerID:         player.PlayerID,
		TeamName:         player.TeamName,
		TeamContactEmail: player.TeamContactEmail,
		DisplayName:      player.DisplayName,
		Email:            player.Email,
		Role:             player.Role,
		SessionVersion:   sessionVersion,
		IssuedAt:         now.UTC().Unix(),
		ExpiresAt:        now.UTC().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}

	headerPart := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsPart := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerPart + "." + claimsPart

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signaturePart := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signaturePart, nil
}

func verifyTeamJWT(secret, token string, now time.Time) (teamTokenClaims, error) {
	if strings.TrimSpace(secret) == "" {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}

	signingInput := parts[0] + "." + parts[1]
	expectedMAC := hmac.New(sha256.New, []byte(secret))
	expectedMAC.Write([]byte(signingInput))
	expectedSignature := expectedMAC.Sum(nil)

	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(providedSignature, expectedSignature) {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}
	if header["alg"] != "HS256" || header["typ"] != "JWT" {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}
	var claims teamTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}
	if claims.PlayerID <= 0 || claims.SessionVersion < 1 || claims.ExpiresAt <= now.UTC().Unix() {
		return teamTokenClaims{}, ErrInvalidTeamToken
	}
	if strings.EqualFold(strings.TrimSpace(claims.Role), "organizer") {
		if claims.TeamID != 0 {
			return teamTokenClaims{}, ErrInvalidTeamToken
		}
		return claims, nil
	}
	return claims, nil
}
