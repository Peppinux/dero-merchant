package webapp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error404Handler handles requests to endpoints that do not exist
func Error404Handler(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
}
