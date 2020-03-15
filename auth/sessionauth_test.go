package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"

	"github.com/peppinux/dero-merchant/config"
	"github.com/peppinux/dero-merchant/redis"
)

type SessionAuthTestSuite struct {
	suite.Suite

	doRequest func(r *gin.Engine, sessionID string) *httptest.ResponseRecorder
}

func (suite *SessionAuthTestSuite) SetupSuite() {
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

	// HTTP request reusable function
	suite.doRequest = func(r *gin.Engine, sessionID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  "DM_SessionID",
			Value: sessionID,
		})
		r.ServeHTTP(w, req)
		return w
	}
}

func (suite *SessionAuthTestSuite) TearDownSuite() {
	redis.FlushAll()
	redis.Pool.Close()
}

func TestSessionAuthTestSuite(t *testing.T) {
	suite.Run(t, new(SessionAuthTestSuite))
}

func (suite *SessionAuthTestSuite) TestSessionAuth() {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(SessionAuth())

	r.POST("/", func(c *gin.Context) {
		s := c.MustGet("session").(*Session)
		c.JSON(http.StatusOK, s)
	})

	// Mock valid session
	userID := 123
	sessionID, hashedSessionID := mockSessionID(userID)

	w := suite.doRequest(r, sessionID)

	var resp *Session
	json.Unmarshal(w.Body.Bytes(), &resp)

	suite.Equal(hashedSessionID, resp.ID)
	suite.True(resp.SignedIn)
	suite.Equal(userID, resp.UserID)

	// Mock unset session
	sessionID, hashedSessionID = mockUnsetSessionID()

	w = suite.doRequest(r, sessionID)

	json.Unmarshal(w.Body.Bytes(), &resp)

	suite.Equal(hashedSessionID, resp.ID)
	suite.False(resp.SignedIn)
	suite.Zero(resp.UserID)
}

func (suite *SessionAuthTestSuite) TestSessionAuthOrRedirect() {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(SessionAuth())
	r.Use(SessionAuthOrRedirect())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Mock valid session
	userID := 123
	sessionID, _ := mockSessionID(userID)

	w := suite.doRequest(r, sessionID)

	suite.Equal(http.StatusOK, w.Code)

	// Mock unset session
	sessionID, _ = mockUnsetSessionID()

	w = suite.doRequest(r, sessionID)

	suite.Equal(http.StatusFound, w.Code)                   // Redirected status code
	suite.Equal("/user/signin", w.Header().Get("Location")) // Redirected to Sign In page
}

func (suite *SessionAuthTestSuite) TestSessionNotAuthOrRedirect() {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(SessionAuth())
	r.Use(SessionNotAuthOrRedirect())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Mock valid session
	userID := 123
	sessionID, _ := mockSessionID(userID)

	w := suite.doRequest(r, sessionID)

	suite.Equal(http.StatusFound, w.Code)                 // Redirected status code
	suite.Equal("/dashboard", w.Header().Get("Location")) // Redirected to Dashboard page

	// Mock unset session
	sessionID, _ = mockUnsetSessionID()

	w = suite.doRequest(r, sessionID)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *SessionAuthTestSuite) TestSessionAuthOrForbidden() {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(SessionAuth())
	r.Use(SessionAuthOrForbidden())

	r.POST("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Mock valid session
	userID := 123
	sessionID, _ := mockSessionID(userID)

	w := suite.doRequest(r, sessionID)

	suite.Equal(http.StatusOK, w.Code)

	// Mock unset session
	sessionID, _ = mockUnsetSessionID()

	w = suite.doRequest(r, sessionID)

	suite.Equal(http.StatusForbidden, w.Code)
}
