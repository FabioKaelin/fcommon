package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/users"
	"github.com/fabiokaelin/ferror"
	"github.com/gin-gonic/gin"
)

type (
	// MinimalUser is a struct for a user with minimal information
	MinimalUser = users.Minimal
)

// GetUserFromContext returns the current user from the context
func GetUserFromContext(c *gin.Context) (MinimalUser, ferror.FError) {
	return users.GetUserFromContext(c)
}
