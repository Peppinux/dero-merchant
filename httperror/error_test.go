package httperror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var testError = &HTTPError{
	Code:    http.StatusForbidden,
	Message: "Forbidden",
}

func doRequest(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestError(t *testing.T) {
	e := NewHTTPError(testError.Code, testError.Message)
	assert.Equal(t, testError, e)

	assert.Equal(t, testError.Message, e.Error())
}

func TestSend(t *testing.T) {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.POST("/send", func(c *gin.Context) {
		Send(c, testError.Code, testError.Message)
	})

	w := doRequest(r, "/send")

	actualBody := w.Body.Bytes()
	var actualBodyJSON = make(map[string]*HTTPError)
	json.Unmarshal(actualBody, &actualBodyJSON)

	expectedBody := map[string]*HTTPError{
		"error": testError,
	}

	assert.Equal(t, testError.Code, w.Code)
	assert.Equal(t, expectedBody, actualBodyJSON)

	r.POST("/send500", func(c *gin.Context) {
		testError := errors.New("Test error")
		Send500(c, testError, "Test error happened")
	})

	w = doRequest(r, "/send500")

	actualBody = w.Body.Bytes()
	actualBodyJSON = make(map[string]*HTTPError)
	json.Unmarshal(actualBody, &actualBodyJSON)

	expectedBody = map[string]*HTTPError{
		"error": &HTTPError{
			Code:    http.StatusInternalServerError,
			Message: "Internal Server Error",
		},
	}

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, expectedBody, actualBodyJSON)

	r.POST("/send500iferr/:shouldErr", func(c *gin.Context) {
		shouldErr := c.Param("shouldErr")
		var testError error
		if shouldErr == "true" {
			testError = errors.New("Test error")
		}
		if Send500IfErr(c, testError, "Test error happened") != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w = doRequest(r, "/send500iferr/true")

	actualBody = w.Body.Bytes()
	actualBodyJSON = make(map[string]*HTTPError)
	json.Unmarshal(actualBody, &actualBodyJSON)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, expectedBody, actualBodyJSON)

	w = doRequest(r, "/send500iferr/false")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRender(t *testing.T) {
	// Setup router
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.LoadHTMLGlob("../webassets/templates/**/*")

	r.POST("/render500", func(c *gin.Context) {
		testError := errors.New("Test error")
		Render500(c, testError, "Test error happened")
	})

	w := doRequest(r, "/render500")

	assert.Equal(t, http.StatusInternalServerError, w.Code) // Only test for response code. Template is excluded.

	r.POST("/render500iferr/:shouldErr", func(c *gin.Context) {
		shouldErr := c.Param("shouldErr")
		var testError error
		if shouldErr == "true" {
			testError = errors.New("Test error")
		}
		if Render500IfErr(c, testError, "Test error happened") != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w = doRequest(r, "/render500iferr/true")

	assert.Equal(t, http.StatusInternalServerError, w.Code) // Only test for response code. Template is excluded.

	w = doRequest(r, "/render500iferr/false")

	assert.Equal(t, http.StatusOK, w.Code)
}
