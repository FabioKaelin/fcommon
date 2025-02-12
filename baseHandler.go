package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/baseHandler"
	"github.com/gin-gonic/gin"
)

func InitBaseHandler(router *gin.Engine, checkOAuthServer bool) {
	baseHandler.InitBaseHandler(router, checkOAuthServer)
}
