package auth

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
)

type SessionTestSuite struct {
	suite.Suite

	mockSessionID      func(userID int) (sessionID, hash string)
	mockUnsetSessionID func() (sessionID, hash string)
}

func (suite *SessionTestSuite) SetupSuite() {
	err := config.LoadFromENV("../.env")
	if err != nil {
		panic(err)
	}

	redis.Pool = redis.NewPool(config.RedisAddress)
	err = redis.Ping()
	if err != nil {
		panic(err)
	}

	err = redis.FlushAll()
	if err != nil {
		panic(err)
	}
}

func (suite *SessionTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()
}

func TestSessionTestSuite(t *testing.T) {
	suite.Run(t, new(SessionTestSuite))
}

func mockSessionID(userID int) (sessionID, hash string) {
	sessionID, _ = GenerateUniqueSessionID()
	hash = cryptoutil.HashStringToSHA256Hex(sessionID)
	redis.SetSessionUser(hash, userID)
	return
}

func mockUnsetSessionID() (sessionID, hash string) {
	sessionID, _ = GenerateUniqueSessionID()
	hash = cryptoutil.HashStringToSHA256Hex(sessionID)
	return
}

func (suite *SessionTestSuite) TestGenerateUniqueSessionID() {
	count := 5
	generatedIDs := make([]string, count)

	for i := 0; i < count; i++ {
		id, err := GenerateUniqueSessionID()
		suite.Nil(err)
		suite.NotContains(generatedIDs, id)

		generatedIDs = append(generatedIDs, id)
	}
}

func (suite *SessionTestSuite) TestGetSessionFromCookie() {
	userID := 123
	sessionID, hashedSessionID := mockSessionID(userID)

	s := GetSessionFromCookie(sessionID)
	suite.Equal(hashedSessionID, s.ID)
	suite.Equal(userID, s.UserID)
	suite.True(s.SignedIn)

	unsetSessionID, hashedUnsetSessionID := mockUnsetSessionID()

	s = GetSessionFromCookie(unsetSessionID)
	suite.Equal(hashedUnsetSessionID, s.ID)
	suite.Zero(s.UserID)
	suite.False(s.SignedIn)

	invalidSessionID, _ := stringutil.RandomBase64RawURLString(49)
	s = GetSessionFromCookie(invalidSessionID)
	suite.Equal(&Session{}, s)
}

func (suite *SessionTestSuite) TestUsername() {
	userID := 123
	username := "foobar"
	redis.SetUserUsername(userID, username)

	sessionID, _ := mockSessionID(userID)

	session := GetSessionFromCookie(sessionID)

	u, err := session.Username()
	suite.Nil(err)
	suite.Equal(username, u)

	unsetSessionID, _ := GenerateUniqueSessionID()
	invalidSession := GetSessionFromCookie(unsetSessionID)

	u, err = invalidSession.Username()
	suite.NotNil(err)
	suite.Zero(u)
}

func (suite *SessionTestSuite) TestEmail() {
	userID := 123
	email := "foo@bar.baz"
	redis.SetUserEmail(userID, email)

	sessionID, _ := mockSessionID(userID)

	session := GetSessionFromCookie(sessionID)

	e, err := session.Email()
	suite.Nil(err)
	suite.Equal(email, e)

	unsetSessionID, _ := GenerateUniqueSessionID()
	invalidSession := GetSessionFromCookie(unsetSessionID)

	e, err = invalidSession.Email()
	suite.NotNil(err)
	suite.Zero(e)
}

func (suite *SessionTestSuite) TestStoresMap() {
	userID := 123
	storesMap := map[int]string{
		2: "Test Store Foo",
		4: "Bar Test Store",
		8: "Baz baz baz 123",
	}

	sessionID, _ := mockSessionID(userID)

	for id, title := range storesMap {
		redis.AddUserStore(userID, id)
		redis.SetStoreTitle(id, title)
	}

	session := GetSessionFromCookie(sessionID)

	stores, err := session.StoresMap()
	suite.Nil(err)
	for id, title := range stores {
		suite.Equal(storesMap[id], title)
	}

	unsetSessionID, _ := GenerateUniqueSessionID()
	invalidSession := GetSessionFromCookie(unsetSessionID)

	stores, _ = invalidSession.StoresMap()
	suite.Equal(map[int]string{}, stores)
}
