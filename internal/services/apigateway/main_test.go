package apigateway

import (
	"os"
	"testing"
)

// TestMain lowers the PBKDF2 work factor for this package's tests. Every
// in-memory store seeds several hashed passwords, and at the production cost
// under -race the package ran into go test's 10-minute timeout in CI. Hashes
// record their own iteration count, so verification is unaffected.
func TestMain(m *testing.M) {
	passwordHashIterations = 1000
	os.Exit(m.Run())
}
