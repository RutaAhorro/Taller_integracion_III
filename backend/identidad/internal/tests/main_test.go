package tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "sup272-test-only-key-at-least-thirty-two-bytes")
	os.Exit(m.Run())
}
