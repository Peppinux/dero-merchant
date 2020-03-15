package auth

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/gin-gonic/gin"

	"github.com/peppinux/dero-merchant/httperror"
	"github.com/peppinux/dero-merchant/postgres"
)

// RequireUserPassword provides a middleware that requires users – authenticated by their Session ID – to provide additional authorization by confirming their password
func RequireUserPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if len(h) == 0 {
			httperror.Send(c, http.StatusBadRequest, "Invalid Authorization header")
			return
		}

		splitPassword := strings.Split(h, " ")
		if len(splitPassword) != 2 || splitPassword[0] != "Password" {
			httperror.Send(c, http.StatusBadRequest, "Invalid Authorization header")
			return
		}

		base64Password := strings.TrimSpace(splitPassword[1])
		passwordBytes, _ := base64.StdEncoding.DecodeString(base64Password)
		password := string(passwordBytes)

		if l := len(password); l < 8 || l > 64 {
			httperror.Send(c, http.StatusUnprocessableEntity, "Password needs to be between 8 and 64 characters long")
			return
		}

		userID := c.MustGet("session").(*Session).UserID

		var hashedPassword string
		err := postgres.DB.QueryRow(`
			SELECT password
			FROM users
			WHERE id=$1 AND email_verified=$2`, userID, true).
			Scan(&hashedPassword)
		if httperror.Send500IfErr(c, err, "Error querying databse") != nil {
			return
		}

		match, err := argon2id.ComparePasswordAndHash(password, hashedPassword)
		if httperror.Send500IfErr(c, err, "Error comparing passwords") != nil {
			return
		}

		if !match {
			httperror.Send(c, http.StatusUnauthorized, "Wrong password")
			return
		}

		c.Next()
	}
}
