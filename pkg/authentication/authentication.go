package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fabiokaelin/fcommon/pkg/logger"
	"github.com/fabiokaelin/fcommon/pkg/notification"
	"github.com/fabiokaelin/fcommon/pkg/users"
	"github.com/fabiokaelin/fcommon/pkg/values"

	"github.com/fabiokaelin/ferror"

	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
)

type (
	// redirect is a struct for the redirect response
	redirect struct {
		Redirect string `json:"redirect"`
	} // @name Redirect

	UserExistFunc   func(string) bool
	GetUserByIDFunc func(string) (users.Minimal, ferror.FError)
	CreateUserFunc  func(users.Minimal) ferror.FError
)

var (
	// function to check if the user exists in the database
	userExist UserExistFunc
	// function to get the user by id
	getUserByID GetUserByIDFunc
	// function to create a user
	createUser CreateUserFunc
)

func SetRequiredFunctions(userExistFunc UserExistFunc, getUserByIDFunc GetUserByIDFunc, createUserFunc CreateUserFunc) {
	userExist = userExistFunc
	getUserByID = getUserByIDFunc
	createUser = createUserFunc
}

// CheckLoginRequest checks if the user is logged in and if not it returns an error this is for automated requests
func CheckLoginRequest() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ""
		cookie, err := ctx.Cookie("token")
		if err != nil {
			tokenHeader := ctx.Request.Header.Get("token")
			if tokenHeader != "" {
				token = tokenHeader
			} else {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, redirect{Redirect: values.V.OAuthFrontendServer + "/login"})
				return
			}
		} else {
			token = cookie
		}

		// TODO: Add cache
		// userData, ok := cache.C.ReadUser(token)
		// if ok {
		// 	ctx.Set("user", userData)
		// 	ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), logger.UserNameKey, userData.Name))
		// 	ctx.Next()
		// 	return
		// }

		client := http.Client{}
		req, err := http.NewRequest("GET", values.V.OAuthBackendInternal+"/api/users/me", nil)
		if err != nil {
			// ctx.AbortWithStatus(204)
			logger.Log.Warn(err.Error())

			ctx.AbortWithStatusJSON(http.StatusUnauthorized, redirect{Redirect: values.V.OAuthFrontendServer + "/login"})

			// ctx.Redirect(http.StatusTemporaryRedirect, "https://oauth.fabkli.ch/login")
			// ctx.JSON(200, "error"+err.Error())
			return
		}

		req.Header = http.Header{
			// "Host":          {"www.host.com"},
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer " + token},
		}

		res, err := client.Do(req)
		if err != nil {
			logger.Log.Warn(err.Error())
			// ctx.Redirect(http.StatusTemporaryRedirect, "https://oauth.fabkli.ch/login") // ?from=" + ctx.Request.URL.Path)
			// ctx.AbortWithStatus(204)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, redirect{Redirect: values.V.OAuthFrontendServer + "/login"})

			// ctx.Redirect(http.StatusTemporaryRedirect, "/login")
			// ctx.JSON(200, "error"+err.Error())
			return
		}
		responseJSON := users.Minimal{}
		json.NewDecoder(res.Body).Decode(&responseJSON)
		responseUser, err := checkUser(responseJSON)
		if err != nil {
			logger.Log.Warn(err.Error())
			// ctx.Redirect(http.StatusTemporaryRedirect, "https://oauth.fabkli.ch/login") // ?from=" + ctx.Request.URL.Path)
			// ctx.AbortWithStatus(204)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, redirect{Redirect: values.V.OAuthFrontendServer + "/login"})
			// ctx.JSON(200, "error"+err.Error())
			return
		}

		// TODO: Add cache
		// cache.C.UpdateUser(token, responseUser)
		ctx.Set("user", responseUser)
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), logger.UserNameKey, responseUser.Name))
		ctx.Next()
	}
}

// CheckLoginUser checks if the user is logged in and if not redirects to the login page this if for manual user requests
func CheckLoginUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ""
		cookie, err := ctx.Cookie("token")
		if err != nil {
			tokenHeader := ctx.Request.Header.Get("token")
			if tokenHeader != "" {
				token = tokenHeader
			} else {
				ctx.Redirect(http.StatusMovedPermanently, values.V.OAuthFrontendServer+`/login?from=`+ctx.Request.Host+ctx.Request.URL.Path)
				return
			}
		} else {
			token = cookie
		}
		client := http.Client{}
		req, err := http.NewRequest("GET", values.V.OAuthBackendInternal+"/api/users/me", nil)
		if err != nil {
			ctx.Redirect(http.StatusMovedPermanently, values.V.OAuthFrontendServer+`/login?from=`+ctx.Request.Host+ctx.Request.URL.Path)

			return
		}

		req.Header = http.Header{
			// "Host":          {"www.host.com"},
			"Content-Type":  {"application/json"},
			"Authorization": {"Bearer " + token},
		}

		res, err := client.Do(req)
		if err != nil {
			logger.Log.Warn(err.Error())
			ctx.Redirect(http.StatusMovedPermanently, values.V.OAuthFrontendServer+`/login?from=`+ctx.Request.Host+ctx.Request.URL.Path)
			return
		}
		responseJSON := users.Minimal{}
		json.NewDecoder(res.Body).Decode(&responseJSON)
		responseUser, err := checkUser(responseJSON)
		if err != nil {
			logger.Log.Warn(err.Error())
			ctx.Redirect(http.StatusMovedPermanently, values.V.OAuthFrontendServer+`/login?from=`+ctx.Request.Host+ctx.Request.URL.Path)
			return
		}

		ctx.Set("user", responseUser)
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), logger.UserNameKey, responseUser.Name))
		ctx.Next()
	}
}

func checkUser(userData users.Minimal) (users.Minimal, error) {
	userExists := userExist(userData.ID)
	if userExists {
		newUserData, ferr := getUserByID(userData.ID)
		if ferr != nil {
			logger.Log.Warn(ferr.Error())
			return users.Minimal{}, ferr
		}
		userData.Privileges = newUserData.Privileges
		// user exist
		return userData, nil
	}
	notificationConfig := notification.Config{Title: "New User", Message: "New User: " + userData.Name + " " + userData.Email + "\n" + spew.Sdump(userData), Type: "newUser"}
	newUserData := users.Minimal{ID: userData.ID, Email: userData.Email, Name: userData.Name, Privileges: 1}
	ferr := notificationConfig.Send()
	if ferr != nil {
		logger.Log.Warn(ferr.Error())
		return users.Minimal{}, ferr
	}
	if newUserData.Email != "" && newUserData.Name != "" {
		ferr = createUser(newUserData)
	} else {
		logger.Log.Warn("no email or name")
		return users.Minimal{}, errors.New("no email or name")
	}
	if ferr != nil {
		if strings.Contains(ferr.Error(), "Duplicate entry") {
			logger.Log.Warn("user already exists")

			time.Sleep(800 * time.Millisecond)

			userExists := userExist(userData.ID)
			if userExists {
				newUserData, ferr := getUserByID(userData.ID)
				if ferr != nil {
					logger.Log.Warn(ferr.Error())
					return users.Minimal{}, ferr
				}
				userData.Privileges = newUserData.Privileges
				// user exist
				return userData, nil
			}
		}
		logger.Log.Warn(ferr.Error())
		return users.Minimal{}, ferr
	}
	userData.Privileges = 0
	return userData, nil
	// user does not exist
}
