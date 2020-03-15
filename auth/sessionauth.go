package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/peppinux/dero-merchant/httperror"
)

// SessionAuth provides a middleware to authenticate a user given their sessionid cookie
func SessionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, _ := c.Cookie("DM_SessionID")
		s := GetSessionFromCookie(cookie)

		c.Set("session", s)
		c.Next()
	}
}

// SessionAuthOrRedirect provides a middleware that redirects user to the Sign In page if they are not authenticated
func SessionAuthOrRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := c.MustGet("session").(*Session)

		if !s.SignedIn {
			c.Redirect(http.StatusFound, "/user/signin")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SessionNotAuthOrRedirect provides a middleware that redirects user to Dashboard if they are already authenticated
func SessionNotAuthOrRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := c.MustGet("session").(*Session)

		if s.SignedIn {
			c.Redirect(http.StatusFound, "/dashboard")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SessionAuthOrForbidden provides a middleware that sends an error message to the user if they are not authenticated
func SessionAuthOrForbidden() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := c.MustGet("session").(*Session)

		if !s.SignedIn {
			httperror.Send(c, http.StatusForbidden, "Invalid session ID")
			return
		}

		c.Next()
	}
}
