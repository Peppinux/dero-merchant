package stringutil

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomBytes(t *testing.T) {
	lengths := []int{0, 1, 10, 100, 1000}

	var generatedBytes [][]byte

	var (
		b   []byte
		err error
	)

	for _, n := range lengths {
		b, err = RandomBytes(n)

		assert.Nil(t, err)
		assert.Equal(t, n, len(b))

		for _, existingBytes := range generatedBytes {
			assert.NotEqual(t, existingBytes, b)
		}

		generatedBytes = append(generatedBytes, b)
	}
}

func TestRandomHexString(t *testing.T) {
	lengths := []int{0, 1, 10, 100, 1000}

	var generatedStrings []string

	var (
		str   string
		bytes []byte
		err   error
	)
	for _, n := range lengths {
		str, err = RandomHexString(n)

		assert.Nil(t, err)

		bytes, err = hex.DecodeString(str)
		assert.Nil(t, err)
		assert.Equal(t, n, len(bytes))

		for _, existingString := range generatedStrings {
			assert.NotEqual(t, existingString, str)
		}

		generatedStrings = append(generatedStrings, str)
	}
}

func TestRandomBase64RawURLString(t *testing.T) {
	lengths := []int{0, 1, 10, 100, 1000}

	var generatedStrings []string

	var (
		str   string
		bytes []byte
		err   error
	)
	for _, n := range lengths {
		str, err = RandomBase64RawURLString(n)

		assert.Nil(t, err)

		bytes, err = base64.RawURLEncoding.DecodeString(str)
		assert.Nil(t, err)
		assert.Equal(t, n, len(bytes))

		for _, existingString := range generatedStrings {
			assert.NotEqual(t, existingString, str)
		}

		generatedStrings = append(generatedStrings, str)
	}
}
