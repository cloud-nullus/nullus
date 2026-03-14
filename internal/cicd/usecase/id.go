package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateID returns a prefixed random ID, e.g. "pip_a1b2c3d4".
func generateID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s_000000000000", prefix)
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
