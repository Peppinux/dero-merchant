package stringutil

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"

	"github.com/pkg/errors"
)

// RandomBytes returns a slice of n cryptographically random bytes
func RandomBytes(n int) (b []byte, err error) {
	b = make([]byte, n)
	_, err = rand.Read(b)
	if err != nil {
		err = errors.Wrap(err, "cannot read random bytes")
	}
	return
}

// RandomHexString returns a new random hex encoded string from n random bytes
func RandomHexString(n int) (string, error) {
	b, err := RandomBytes(n)
	return hex.EncodeToString(b), err
}

// RandomBase64RawURLString returns a new random base64 raw url encoded string from n random bytes
func RandomBase64RawURLString(n int) (string, error) {
	b, err := RandomBytes(n)
	return base64.RawURLEncoding.EncodeToString(b), err
}
