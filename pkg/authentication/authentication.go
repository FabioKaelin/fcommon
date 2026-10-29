package authentication

import (
	"context"
	"errors"
	"github.com/fabiokaelin/fcommon/pkg/oauthsession"
	"github.com/fabiokaelin/fcommon/pkg/ratelimit"
	"net/http"
	"net/url"
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

var sessionClient = oauthsession.NewClient()

// CheckLoginRequest validates and renews a session on authenticated API activity.
func CheckLoginRequest() gin.HandlerFunc { return checkLogin(false) }
func CheckLoginUser() gin.HandlerFunc    { return checkLogin(true) }

func checkLogin(redirectBrowser bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reject := func(status int) {
			if redirectBrowser && status == http.StatusUnauthorized {
				from := "https://" + ctx.Request.Host + ctx.Request.URL.RequestURI()
				ctx.Redirect(http.StatusSeeOther, values.V.OAuthFrontendServer+"/login?from="+url.QueryEscape(from))
			} else {
				ctx.AbortWithStatusJSON(status, redirect{Redirect: values.V.OAuthFrontendServer + "/login"})
			}
		}
		token := ""
		cookies := ctx.Request.CookiesNamed("token")
		if len(cookies) > 1 {
			reject(http.StatusUnauthorized)
			return
		}
		if len(cookies) == 1 {
			if ctx.Request.Method != "GET" && ctx.Request.Method != "HEAD" && ctx.Request.Method != "OPTIONS" {
				origin := ctx.GetHeader("Origin")
				if origin == "" {
					if ref, err := url.Parse(ctx.Request.Referer()); err == nil && ref.Host != "" {
						origin = ref.Scheme + "://" + ref.Host
					}
				}
				if origin != "https://"+ctx.Request.Host {
					ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Cross-origin write forbidden"})
					return
				}
			}
			token = cookies[0].Value
		} else {
			token = ctx.GetHeader("token")
		}
		result, err := sessionClient.Validate(ctx.Request.Context(), values.V.OAuthBackendInternal+"/api/users/me", token)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, oauthsession.ErrUnauthenticated) {
				status = http.StatusUnauthorized
			}
			reject(status)
			return
		}
		responseJSON := users.Minimal{ID: result.User.ID, Name: result.User.Name, Email: result.User.Email, Privileges: result.User.Privileges}
		responseUser, err := checkUser(responseJSON)
		if err != nil {
			logger.Log.Warn(err.Error())
			reject(http.StatusUnauthorized)
			return
		}
		if !ratelimit.User(ctx, responseUser.ID) {
			return
		}
		result.ForwardCookies(ctx.Writer)
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
