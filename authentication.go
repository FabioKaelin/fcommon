package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/authentication"
	"github.com/gin-gonic/gin"
)

func InitAuth(userExistFunc authentication.UserExistFunc, getUserByIDFunc authentication.GetUserByIDFunc, createUserFunc authentication.CreateUserFunc) {
	authentication.SetRequiredFunctions(userExistFunc, getUserByIDFunc, createUserFunc)
}

// CheckLoginRequest checks if the user is logged in and if not it returns an error this is for automated requests
func CheckLoginRequest() gin.HandlerFunc {
	return authentication.CheckLoginRequest()
}

// CheckLoginUser checks if the user is logged in and if not redirects to the login page this if for manual user requests
func CheckLoginUser() gin.HandlerFunc {
	return authentication.CheckLoginUser()
}
