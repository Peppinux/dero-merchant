package httperror

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// HTTPError holds an error message and its status code
type HTTPError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewHTTPError returns a new HTTPError
func NewHTTPError(code int, message string) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: message,
	}
}

func (e *HTTPError) Error() string {
	return e.Message
}

// Send sends an HTTPError to a Gin request and aborts
func Send(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{
		"error": NewHTTPError(code, message),
	})
}

// Send500 sends a generic Internal Server Error HTTPError response to a Gin request
func Send500(c *gin.Context, err error, description string) {
	c.Error(errors.Wrap(err, description)) // Actual error is only logged and not sent to the response
	Send(c, 500, "Internal Server Error")
}

// Send500IfErr executes Send500 only if err != nil and returns err
func Send500IfErr(c *gin.Context, err error, description string) error {
	if err != nil {
		Send500(c, err, description)
		return err
	}

	return nil
}

// Render500 renders a generic Internal Server Error page
func Render500(c *gin.Context, err error, description string) {
	c.HTML(http.StatusInternalServerError, "500.html", nil)
	c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, description)) // Actual error is only logged and not sent to the response
}

// Render500IfErr executes Render500 only if err != nil and returns err
func Render500IfErr(c *gin.Context, err error, description string) error {
	if err != nil {
		Render500(c, err, description)
		return err
	}
	return nil
}
