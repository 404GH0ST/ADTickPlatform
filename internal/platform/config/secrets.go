package config

import (
	"log"
	"strings"
)

var rejectedSecretValues = map[string]struct{}{
	"dev-team-token":                                          {},
	"dev-admin-token":                                         {},
	"dev-flag-secret":                                         {},
	"change-this-admin-token":                                 {},
	"change-this-team-jwt-secret":                             {},
	"change-this-unlock-proof-secret":                         {},
	"change-this-stable-ssh-credential-secret":                {},
	"change-this-flag-secret":                                 {},
	"replace-with-a-long-random-admin-token":                  {},
	"replace-with-a-long-random-team-jwt-secret":              {},
	"replace-with-a-long-random-unlock-proof-secret":          {},
	"replace-with-a-long-random-stable-ssh-secret":            {},
	"replace-with-a-long-random-stable-ssh-credential-secret": {},
	"replace-with-a-long-random-flag-secret":                  {},
	"replace-with-a-real-private-key":                         {},
}

func RequiredSecret(key string) string {
	value := strings.TrimSpace(String(key, ""))
	return validateSecret(key, value)
}

func validateSecret(key, value string) string {
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	if _, rejected := rejectedSecretValues[value]; rejected {
		log.Fatalf("%s must not use the example or development value", key)
	}
	if strings.HasPrefix(value, "change-this-") || strings.HasPrefix(value, "replace-with-") {
		log.Fatalf("%s must not use the example or development value", key)
	}
	return value
}

func Secret(key, fallbackKey string) string {
	value := strings.TrimSpace(String(key, ""))
	if value != "" {
		return validateSecret(key, value)
	}
	if fallbackKey != "" {
		return RequiredSecret(fallbackKey)
	}
	return RequiredSecret(key)
}
