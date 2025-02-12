package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/logger"
	"github.com/gin-gonic/gin"
)

func GetGinLogger(ginMode string, jsonLogs bool) gin.HandlerFunc {
	return logger.GetGinLogger(ginMode, jsonLogs)
}
