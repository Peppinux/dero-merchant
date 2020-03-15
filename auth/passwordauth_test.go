package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/stringutil"
	"github.com/stretchr/testify/suite"
)

type PasswordAuthUserMock struct {
	ID       int
	Username string
	Email    string
	Password string
	Verified bool

	HashedPassword    string
	VerificationToken string

	Base64Password string
	AuthHeader     string

	SessionID string
}

type PasswordAuthTestSuite struct {
	suite.Suite

	mockUsers map[string]*PasswordAuthUserMock

	doRequest func(r *gin.Engine, sessionID, authHeader string) *httptest.ResponseRecorder
}

func (suite *PasswordAuthTestSuite) SetupSuite() {
	err := config.LoadFromENV("../.env")
	if err != nil {
		panic(err)
	}

	redis.Pool = redis.NewPool(config.TestRedisAddress)
	err = redis.Ping()
	if err != nil {
		panic(err)
	}

	err = redis.FlushAll()
	if err != nil {
		panic(err)
	}

	postgres.DB, err = postgres.Connect(config.TestDBName, config.TestDBUser, config.TestDBPassword, config.TestDBHost, config.TestDBPort, "disable") // TODO: Enable SSLMode?
	if err != nil {
		panic(err)
	}

	postgres.DropTables()
	postgres.CreateTablesIfNotExist()

	suite.mockUsers = map[string]*PasswordAuthUserMock{
		"valid": {
			Username: "Valid User",
			Email:    "foo@bar.baz",
			Password: "foobarbaz",
			Verified: true,
		},
		"unverified": {
			Username: "Not Verified",
			Email:    "test@test.com",
			Password: "password",
			Verified: false,
		},
		"inexistent": {
			Username: "I Do Not Exist",
			Email:    "fake@email.it",
			Password: "123456789",
		},
	}

	for _, u := range suite.mockUsers {
		u.HashedPassword, _ = argon2id.CreateHash(u.Password, argon2id.DefaultParams)
		u.VerificationToken, _ = stringutil.RandomBase64RawURLString(48)

		postgres.DB.QueryRow(`
			INSERT INTO users (username, email, password, verification_token, email_verified)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`, u.Username, u.Email, u.HashedPassword, u.VerificationToken, u.Verified).
			Scan(&u.ID)
	}

	// Mock valid user's session
	suite.mockUsers["valid"].SessionID, _ = mockSessionID(suite.mockUsers["valid"].ID)

	// HTTP request reusable function
	suite.doRequest = func(r *gin.Engine, sessionID, authHeader string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/", nil)

		if sessionID != "" {
			req.AddCookie(&http.Cookie{
				Name:  "DM_SessionID",
				Value: sessionID,
			})
		}

		if authHeader != "" {
			req.Header.Add("Authorization", authHeader)
			r.ServeHTTP(w, req)
		}

		r.ServeHTTP(w, req)

		return w
	}
}

func (suite *PasswordAuthTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()

	postgres.DropTables()
	postgres.DB.Close()
}

func TestPasswordAuthTestSuite(t *testing.T) {
	suite.Run(t, new(PasswordAuthTestSuite))
}

func (suite *PasswordAuthTestSuite) TestRequireUserPassword() {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(SessionAuth())
	r.Use(SessionAuthOrForbidden())
	r.Use(RequireUserPassword())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	users := suite.mockUsers

	// Valid user with valid password, unverified user and inexistent user
	for k, u := range users {
		u.Base64Password = base64.StdEncoding.EncodeToString([]byte(u.Password))
		u.AuthHeader = fmt.Sprintf("Password %s", u.Base64Password)

		w := suite.doRequest(r, u.SessionID, u.AuthHeader)

		switch k {
		case "valid":
			suite.Equal(http.StatusOK, w.Code) // Request went through
		case "unverified", "inexistent":
			suite.Equal(http.StatusForbidden, w.Code) // Request rejected by SessionAuthOrForbidden middleware
		}
	}

	u := users["valid"]

	// Valid user with valid Authorization token but no Session cookie
	w := suite.doRequest(r, "", u.AuthHeader)

	suite.Equal(http.StatusForbidden, w.Code) // Request rejected by SessionAuthOrForbidden middelware

	// Valid user with invalid Authorization token
	w = suite.doRequest(r, u.SessionID, "")

	suite.Equal(http.StatusBadRequest, w.Code) // Request rejected by RequireUserPassword middlware because of no Authorization header

	invalidHeader := fmt.Sprintf("ThisIsNotPassword %s", u.Base64Password)
	w = suite.doRequest(r, u.SessionID, invalidHeader)

	suite.Equal(http.StatusBadRequest, w.Code) // Request rejected by RequireUserPassword middlware because of invalid Authorization header

	invalidPass := base64.StdEncoding.EncodeToString([]byte("passwor")) // Less than 8 chars
	invalidHeader = fmt.Sprintf("Password %s", invalidPass)
	w = suite.doRequest(r, u.SessionID, invalidHeader)

	suite.Equal(http.StatusUnprocessableEntity, w.Code) // Request rejected by RequireUserPassword middleware because of invalid password format

	// Valid user with wrong password
	wrongPass := base64.StdEncoding.EncodeToString([]byte("foobarba")) // Misses last char
	invalidHeader = fmt.Sprintf("Password %s", wrongPass)
	w = suite.doRequest(r, u.SessionID, invalidHeader)

	suite.Equal(http.StatusUnauthorized, w.Code) // Request rejected by RequirePasswordMiddleware because of invalid password format
}
