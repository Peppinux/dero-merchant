package cryptoutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashStringToSHA256Hex returns the hex encoded SHA256 checksum of a given string
func HashStringToSHA256Hex(str string) string {
	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:])
}
