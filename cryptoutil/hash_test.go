package cryptoutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashStringToSHA256Hex(t *testing.T) {
	tests := []struct {
		String       string
		ExpectedHash string
	}{
		{
			String:       "",
			ExpectedHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			String:       "foo",
			ExpectedHash: "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
		},
		{
			String:       "bar",
			ExpectedHash: "fcde2b2edba56bf408601fb721fe9b5c338d10ee429ea04fae5511b68fbf8fb9",
		},
		{
			String:       "foobar",
			ExpectedHash: "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2",
		},
	}

	var actual string
	for _, test := range tests {
		actual = HashStringToSHA256Hex(test.String)
		assert.Equal(t, test.ExpectedHash, actual)
	}
}
