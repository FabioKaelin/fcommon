package users

import (
	"github.com/fabiokaelin/ferror"
	"github.com/gin-gonic/gin"
)

// GetUserFromContext returns the current user from the context
func GetUserFromContext(c *gin.Context) (Minimal, ferror.FError) {
	userData, exist := c.Get("user")
	if !exist {
		ferr := ferror.New("user not found")
		ferr.SetLayer("middleware")
		ferr.SetKind("get current user")
		ferr.SetInternal("user not found in gin context")
		return Minimal{}, ferr
	}
	// var userResponse UserResponse
	userResponse := userData.(Minimal)
	return userResponse, nil
}

// Minimal is a struct for a user with minimal information
type Minimal struct {
	ID         string `json:"id,omitempty" example:"9bf3e317-77a0-4643-be11-5eacd4c630ce"` // UUID
	Name       string `json:"name,omitempty" example:"Fabio Kälin"`                        // Username
	Email      string `json:"email,omitempty" example:"fabio.kaelin@fabkli.ch"`            // Email
	Privileges int    `json:"privileges,omitempty" example:"13"`                           // Privileges
} // @name UserMinimal
