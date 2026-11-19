package apigateway

import "encoding/hex"

func newCheckerToken() string {
	return hex.EncodeToString(randomBytes(32))
}
