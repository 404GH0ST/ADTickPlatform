package apigateway

import (
	"crypto/hmac"
	"crypto/rand"
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
	TeamName         string `json:"team_name,omitempty"`
	TeamContactEmail string `json:"team_contact_email,omitempty"`
	DisplayName      string `json:"display_name,omitempty"`
	Email            string `json:"email,omitempty"`
	Role             string `json:"role"`
	SessionVersion   int    `json:"session_version"`
	SessionID        string `json:"jti"`
	IssuedAt         int64  `json:"iat"`
	ExpiresAt        int64  `json:"exp"`
}

// teamJWTHeaderPart is the encoded {"alg":"HS256","typ":"JWT"} header every
// team token carries.
var teamJWTHeaderPart = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

// issueToken is a seam over issueTeamJWT so handlers' token-failure responses
// can be tested; signing only fails for an empty secret, which the session
// check rejects first.
var issueToken = issueTeamJWT

func issueTeamJWT(secret string, player authenticatedPlayer, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("team token secret is empty")
	}
	player = canonicalSessionPlayer(player)

	sessionVersion := player.SessionVersion
	if sessionVersion < 1 {
		sessionVersion = 1
	}
	if player.sessionID == "" {
		player.sessionID = rand.Text()
		player.sessionExpiresAt = now.Add(24 * time.Hour).Unix()
	}
	if player.sessionExpiresAt <= now.Unix() {
		return "", ErrInvalidTeamToken
	}
	// The claims are plain strings and integers, which always marshal.
	claims := teamTokenClaims{
		TeamID:           player.TeamID,
		PlayerID:         player.PlayerID,
		TeamName:         player.TeamName,
		TeamContactEmail: player.TeamContactEmail,
		DisplayName:      player.DisplayName,
		Email:            player.Email,
		Role:             player.Role,
		SessionVersion:   sessionVersion,
		SessionID:        player.sessionID,
		IssuedAt:         now.UTC().Unix(),
		ExpiresAt:        player.sessionExpiresAt,
	}
	claimsJSON, _ := json.Marshal(claims)
	// A legacy profile can still exceed browser cookie limits after JSON
	// escaping and base64. Session validation loads the current profile.
	const maxCookieClaimsBytes = 2700
	if len(claimsJSON) > maxCookieClaimsBytes {
		claims.TeamName = ""
		claims.TeamContactEmail = ""
		claims.DisplayName = ""
		claims.Email = ""
		claimsJSON, _ = json.Marshal(claims)
	}

	claimsPart := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := teamJWTHeaderPart + "." + claimsPart

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
	if claims.SessionID == "" {
		// Sessions signed before jti was added can still be revoked. The
		// signing input identifies the token even if its signature is re-encoded.
		claims.SessionID = "legacy:" + hashSecret(signingInput)
	}
	if strings.EqualFold(strings.TrimSpace(claims.Role), "organizer") {
		if claims.TeamID != 0 {
			return teamTokenClaims{}, ErrInvalidTeamToken
		}
		return claims, nil
	}
	return claims, nil
}
