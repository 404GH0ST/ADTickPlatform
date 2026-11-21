package apigateway

import "strings"

// Challenge categories. The empty string is "uncategorized": challenges that
// predate categories keep it until an organizer picks one.
const (
	challengeCategoryNone   = ""
	challengeCategoryPwn    = "pwn"
	challengeCategoryWeb    = "web"
	challengeCategoryCrypto = "crypto"
)

// Connection protocols participants use to reach a service. TCP is the
// default because every service is reachable as raw TCP.
const (
	challengeProtocolTCP  = "tcp"
	challengeProtocolHTTP = "http"
)

// normalizeChallengeCategory lowercases and trims raw. ok is false for a value
// outside the known set; an empty value is valid and means uncategorized.
func normalizeChallengeCategory(raw string) (string, bool) {
	switch value := strings.ToLower(strings.TrimSpace(raw)); value {
	case challengeCategoryNone, challengeCategoryPwn, challengeCategoryWeb, challengeCategoryCrypto:
		return value, true
	default:
		return "", false
	}
}

// normalizeChallengeProtocol lowercases and trims raw, defaulting an empty
// value to TCP. ok is false for any other protocol.
func normalizeChallengeProtocol(raw string) (string, bool) {
	switch value := strings.ToLower(strings.TrimSpace(raw)); value {
	case "":
		return challengeProtocolTCP, true
	case challengeProtocolTCP, challengeProtocolHTTP:
		return value, true
	default:
		return "", false
	}
}

// storedChallengeCategory and storedChallengeProtocol are what a store
// persists. Handlers reject invalid input first; these fall back to the
// defaults so a direct store caller can never write a value the database
// constraint would refuse.
func storedChallengeCategory(raw string) string {
	value, ok := normalizeChallengeCategory(raw)
	if !ok {
		return challengeCategoryNone
	}
	return value
}

func storedChallengeProtocol(raw string) string {
	value, ok := normalizeChallengeProtocol(raw)
	if !ok {
		return challengeProtocolTCP
	}
	return value
}

// validateChallengeMetadata checks the optional category and protocol of a
// create or update request. A nil pointer means "not sent".
func validateChallengeMetadata(category, protocol *string) (string, bool) {
	if category != nil {
		if _, ok := normalizeChallengeCategory(*category); !ok {
			return "category must be one of pwn, web or crypto.", false
		}
	}
	if protocol != nil {
		if _, ok := normalizeChallengeProtocol(*protocol); !ok {
			return "protocol must be tcp or http.", false
		}
	}
	return "", true
}
