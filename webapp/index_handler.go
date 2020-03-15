package webapp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/peppinux/dero-merchant/auth"
)

type indexData struct {
	UserSignedIn bool
	Username     string
}

// IndexHandler handles GET requests to /
func IndexHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)
	username, _ := s.Username()

	resp := &indexData{
		UserSignedIn: s.SignedIn,
		Username:     username,
	}
	c.HTML(http.StatusOK, "index.html", resp)
}
