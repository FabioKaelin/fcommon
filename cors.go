package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/cors"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return cors.CORSMiddleware()
}
