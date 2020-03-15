package auth

import (
	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

// Session represents the cookie session of a user
type Session struct {
	ID       string
	SignedIn bool
	UserID   int
}

// GetSessionFromCookie returns a new user Session loaded from Redis through the sessionid cookie
func GetSessionFromCookie(cookie string) (s *Session) {
	s = &Session{}

	if len(cookie) != 64 {
		return
	}

	s.ID = cryptoutil.HashStringToSHA256Hex(cookie)

	var err error
	s.UserID, err = redis.GetSessionUser(s.ID)
	if err != nil {
		s.SignedIn = false
	} else {
		s.SignedIn = true
	}

	return
}

// Username returns the username of the user associated to the session
func (s *Session) Username() (username string, err error) {
	username, err = redis.GetUserUsername(s.UserID)
	if err != nil {
		err = errors.Wrap(err, "cannot get user's username from Redis")
	}
	return
}

// Email returns the email of the user associated to the session
func (s *Session) Email() (email string, err error) {
	email, err = redis.GetUserEmail(s.UserID)
	if err != nil {
		err = errors.Wrap(err, "cannot get user's email from Redis")
	}
	return
}

// StoresMap returns a map of the stores (ID: Title) of the user associated to the session
func (s *Session) StoresMap() (storesMap map[int]string, err error) {
	stores, err := redis.GetUserStores(s.UserID)
	if err != nil {
		err = errors.Wrap(err, "cannot get user's stores from Redis")
		return
	}

	storesMap = make(map[int]string, len(stores))

	for _, storeID := range stores {
		storesMap[storeID], err = redis.GetStoreTitle(storeID)
		if err != nil {
			err = errors.Wrap(err, "cannot get store's title")
			return
		}
	}

	return
}

func generateSessionID() (string, error) {
	return stringutil.RandomBase64RawURLString(48)
}

// GenerateUniqueSessionID generates a unique Session ID
func GenerateUniqueSessionID() (sessionID string, err error) {
	for {
		// Generate Session ID
		sessionID, err = generateSessionID()
		if err != nil {
			err = errors.Wrap(err, "cannot generate session ID")
			return
		}

		// Get User ID associated to generated Session ID
		userID, _ := redis.GetSessionUser(sessionID)

		// If NO User ID is found, generated Session ID is unique, therefore return its value
		if userID == 0 {
			return
		}
	}
}
