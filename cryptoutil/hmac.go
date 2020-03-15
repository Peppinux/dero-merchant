package cryptoutil

import (
	"crypto/hmac"
	"crypto/sha256"

	"github.com/pkg/errors"
)

// SignMessage returns the signature of message signed with key
func SignMessage(message, key []byte) (signature []byte, err error) {
	mac := hmac.New(sha256.New, key)
	_, err = mac.Write(message)
	if err != nil {
		err = errors.Wrap(err, "cannot write message to mac")
		return
	}

	signature = mac.Sum(nil)
	return
}

// ValidMAC reports whether messageMAC is a valid HMAC tag for message
func ValidMAC(message, messageMAC, key []byte) (valid bool, err error) {
	signedMessage, err := SignMessage(message, key)
	if err != nil {
		err = errors.Wrap(err, "cannot sign message")
		return
	}
	valid = hmac.Equal(messageMAC, signedMessage)
	return
}
