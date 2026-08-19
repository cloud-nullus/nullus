package keycloak

import (
	"os"
	"strings"
	"testing"
)

func readKeycloakSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s 를 읽지 못했다: %v", name, err)
	}
	return string(b)
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
