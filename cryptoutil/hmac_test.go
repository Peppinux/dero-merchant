package cryptoutil

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignMessage(t *testing.T) {
	tests := []struct {
		Message              string
		HexKey               string
		ExpectedHexSignature string
	}{
		{
			Message:              "",
			HexKey:               "844386547bcc37ad4f54b5b3cd37043f18f0004944eeeb214c6d51783894a2a3",
			ExpectedHexSignature: "c351dfd04241bcc7832a6079233a9ea13044e100c5cd52f44acf9d2e5a73083d",
		},
		{
			Message:              "foo",
			HexKey:               "dd1293c94deb8d51c059f15d9632df5732841ae48205990c6b8a729561fb8320",
			ExpectedHexSignature: "aadd5e36a6c5ce8ac8e9b407cc5ebcd9647b2a1d2b2591a50547c46229a7d1ed",
		},
		{
			Message:              "bar",
			HexKey:               "f05d086f2a6882af5a2fac20ac42c33479aefa87324cc69b53c02765ec850920",
			ExpectedHexSignature: "f4f35fde23d67e9b999da0fe97ba170ff715f50fab624efd7a2f7570ea05483a",
		},
		{
			Message:              "foobar",
			HexKey:               "09725d9dcc8d50c6e6cd9e804713946506881d92b300c07b592359df7150ab96",
			ExpectedHexSignature: "3e3d2c58a5738f41e6d338c482915255cacf5aa8ab6d116305f174a7b18834d7",
		},
	}

	var (
		msg               []byte
		key               []byte
		expectedSignature []byte
		signature         []byte
		err               error
	)
	for _, test := range tests {
		msg = []byte(test.Message)
		key, _ = hex.DecodeString(test.HexKey)
		expectedSignature, _ = hex.DecodeString(test.ExpectedHexSignature)

		signature, err = SignMessage(msg, key)

		assert.Nil(t, err)
		assert.Equal(t, expectedSignature, signature)
	}
}

func TestValidMAC(t *testing.T) {
	tests := []struct {
		Message          string
		HexMessageKey    string
		HexMessageMAC    string
		ExpectedValidity bool
	}{
		{Message: "",
			HexMessageKey:    "844386547bcc37ad4f54b5b3cd37043f18f0004944eeeb214c6d51783894a2a3",
			HexMessageMAC:    "c351dfd04241bcc7832a6079233a9ea13044e100c5cd52f44acf9d2e5a73083d",
			ExpectedValidity: true,
		},
		{Message: "foo",
			HexMessageKey:    "dd1293c94deb8d51c059f15d9632df5732841ae48205990c6b8a729561fb8320",
			HexMessageMAC:    "aadd5e36a6c5ce8ac8e9b407cc5ebcd9647b2a1d2b2591a50547c46229a7d1ed",
			ExpectedValidity: true,
		},
		{Message: "bar",
			HexMessageKey:    "f05d086f2a6882af5a2fac20ac42c33479aefa87324cc69b53c02765ec850920",
			HexMessageMAC:    "c351dfd04241bcc7832a6079233a9ea13044e100c5cd52f44acf9d2e5a73083d",
			ExpectedValidity: false,
		},
		{Message: "foobar",
			HexMessageKey:    "09725d9dcc8d50c6e6cd9e804713946506881d92b300c07b592359df7150ab96",
			HexMessageMAC:    "aadd5e36a6c5ce8ac8e9b407cc5ebcd9647b2a1d2b2591a50547c46229a7d1ed",
			ExpectedValidity: false,
		},
	}

	var (
		msg    []byte
		msgKey []byte
		msgMAC []byte
		valid  bool
		err    error
	)
	for _, test := range tests {
		msg = []byte(test.Message)
		msgKey, _ = hex.DecodeString(test.HexMessageKey)
		msgMAC, _ = hex.DecodeString(test.HexMessageMAC)

		valid, err = ValidMAC(msg, msgMAC, msgKey)

		assert.Nil(t, err)
		assert.Equal(t, test.ExpectedValidity, valid)
	}
}
