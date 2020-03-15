package redis

import (
	"testing"
	"time"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/stringutil"
	"github.com/stretchr/testify/suite"
)

type ActionsTestSuite struct {
	suite.Suite
}

func (suite *ActionsTestSuite) SetupSuite() {
	err := config.LoadFromENV("../.env")
	if err != nil {
		panic(err)
	}

	Pool = NewPool(config.RedisAddress)
	err = Ping()
	if err != nil {
		panic(err)
	}

	err = FlushAll()
	if err != nil {
		panic(err)
	}
}

func (suite *ActionsTestSuite) TearDownSuite() {
	FlushAll()
	Pool.Close()
}

func TestActionsTestSuite(t *testing.T) {
	suite.Run(t, new(ActionsTestSuite))
}

func (suite *ActionsTestSuite) TestSessionUser() {
	sessionID, _ := stringutil.RandomBase64RawURLString(48)
	userID := 123

	unsetSessionID, _ := stringutil.RandomBase64RawURLString(48)

	// Set
	err := SetSessionUser(sessionID, userID)
	suite.Nil(err)

	// Get
	uid, err := GetSessionUser(sessionID)
	suite.Nil(err)
	suite.Equal(userID, uid)

	uid, err = GetSessionUser(unsetSessionID)
	suite.NotNil(err)
	suite.Zero(uid)

	// Delete
	err = DeleteSession(sessionID)
	suite.Nil(err)
	uid, err = GetSessionUser(sessionID)
	suite.NotNil(err)
	suite.Zero(uid)

	// Reset to test Expire
	SetSessionUser(sessionID, userID)

	// Expire
	err = SetSessionExpiration(sessionID, 2)
	suite.Nil(err)
	uid, err = GetSessionUser(sessionID)
	suite.Nil(err)
	suite.Equal(userID, uid)
	time.AfterFunc(time.Second*2, func() {
		uid, err = GetSessionUser(sessionID)
		suite.NotNil(err)
		suite.Zero(uid)
	})
}

func (suite *ActionsTestSuite) TestUserSessions() {
	userID := 123
	sessionIDs := make(map[string]bool)
	var removedSessionID string
	for i := 0; i < 5; i++ {
		s, _ := stringutil.RandomBase64RawURLString(48)
		if i == 0 {
			removedSessionID = s
		}
		if i < 3 {
			sessionIDs[s] = true
		} else {
			sessionIDs[s] = false
		}
	}

	// Add
	for k, v := range sessionIDs {
		if v == true {
			err := AddUserSession(userID, k)
			suite.Nil(err)
		}
	}

	// Get
	sessions, err := GetUserSessions(userID)
	suite.Nil(err)
	for _, s := range sessions {
		_, hasKey := sessionIDs[s]
		suite.Equal(true, hasKey)
	}

	// Remove
	err = RemoveUserSession(userID, removedSessionID)
	suite.Nil(err)
	sessions, _ = GetUserSessions(userID)
	for _, s := range sessions {
		suite.NotEqual(removedSessionID, s)
	}

	// Delete
	err = DeleteUserSessions(userID)
	suite.Nil(err)
	sessions, err = GetUserSessions(userID)
	suite.Equal([]string{}, sessions)
}

func (suite *ActionsTestSuite) TestUserUsername() {
	userID := 123
	username := "foobar"

	// Set
	err := SetUserUsername(userID, username)
	suite.Nil(err)

	// Get
	u, err := GetUserUsername(userID)
	suite.Nil(err)
	suite.Equal(username, u)
}

func (suite *ActionsTestSuite) TestUserEmail() {
	userID := 123
	email := "foo@bar.com"

	// Set
	err := SetUserEmail(userID, email)
	suite.Nil(err)

	// Get
	e, err := GetUserEmail(userID)
	suite.Nil(err)
	suite.Equal(email, e)
}

func (suite *ActionsTestSuite) TestUserStores() {
	userID := 123
	storeIDs := map[int]bool{
		2:  true,
		4:  true,
		8:  true,
		16: false,
		32: false,
	}

	for id, owned := range storeIDs {
		// Add
		if owned {
			err := AddUserStore(userID, id)
			suite.Nil(err)
		}

		// Own
		owns, err := UserOwnsStore(userID, id)
		suite.Nil(err)
		if owned {
			suite.True(owns)
		} else {
			suite.False(owns)
		}
	}

	// Get
	stores, err := GetUserStores(userID)
	suite.Nil(err)
	for _, id := range stores {
		suite.True(storeIDs[id])
	}

	// Remove
	removedStoreID := 4
	err = RemoveUserStore(userID, removedStoreID)
	suite.Nil(err)
	stores, _ = GetUserStores(userID)
	suite.NotContains(stores, removedStoreID)
}

func (suite *ActionsTestSuite) TestStoreTitle() {
	storeID := 8
	title := "Test foo bar baz"

	// Set
	err := SetStoreTitle(storeID, title)
	suite.Nil(err)

	// Get
	actual, err := GetStoreTitle(storeID)
	suite.Nil(err)
	suite.Equal(title, actual)

	// Delete
	err = DeleteStoreTitle(storeID)
	suite.Nil(err)
	actual, err = GetStoreTitle(storeID)
	suite.NotNil(err)
	suite.Zero(actual)
}

func (suite *ActionsTestSuite) TestAPIKeyStore() {
	apiKey, _ := stringutil.RandomHexString(32)
	storeID := 8

	// Set
	err := SetAPIKeyStore(apiKey, storeID)
	suite.Nil(err)

	// Get
	id, err := GetAPIKeyStore(apiKey)
	suite.Nil(err)
	suite.Equal(storeID, id)

	// Delete
	err = DeleteAPIKeyStore(apiKey)
	suite.Nil(err)
	id, err = GetAPIKeyStore(apiKey)
	suite.NotNil(err)
	suite.Zero(id)
}

func (suite *ActionsTestSuite) TestAPIKeySecretKey() {
	apiKey, _ := stringutil.RandomHexString(32)
	secretKey, _ := stringutil.RandomHexString(32)

	// Set
	err := SetAPIKeySecretKey(apiKey, secretKey)
	suite.Nil(err)

	// Get
	sk, err := GetAPIKeySecretKey(apiKey)
	suite.Nil(err)
	suite.Equal(secretKey, sk)

	// Delete
	err = DeleteAPIKeySecretKey(apiKey)
	suite.Nil(err)
	sk, err = GetAPIKeySecretKey(apiKey)
	suite.NotNil(err)
	suite.Zero(sk)
}

func (suite *ActionsTestSuite) TestSupportedCurrencies() {
	supportedCurrencies := []string{"usd", "eur", "btc"}
	unsupportedCurrencies := []string{"asd", "esd", "isd"}

	// Set
	err := SetSupportedCurrencies(supportedCurrencies)
	suite.Nil(err)

	// Is
	for _, c := range supportedCurrencies {
		isSupported, err := IsSupportedCurrency(c)
		suite.Nil(err)
		suite.True(isSupported)
	}

	for _, c := range unsupportedCurrencies {
		isSupported, err := IsSupportedCurrency(c)
		suite.Nil(err)
		suite.False(isSupported)
	}
}
