package fcommon

import (
	"github.com/fabiokaelin/fcommon/internal/logger"
)

var Log = logger.Log

// InitLogger initializes the logger with the given configuration.
func InitLogger(ginMode string, jsonLogs bool) {
	logger.InitZapCustomLogger(ginMode, jsonLogs)
}
