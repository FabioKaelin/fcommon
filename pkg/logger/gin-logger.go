package logger

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

var (
	// loggerConfig is the config for the logger
	loggerConfig = gin.LoggerConfig{
		Formatter: defaultLogFormatter,
		// Skip: func(c *gin.Context) bool {
		// 	return strings.HasPrefix(c.Request.URL.Path, "/internal")
		// },
	}
	colorGin = color.New(color.BgHiBlue, color.FgHiWhite).Sprint("[GIN]")
)

// contextKeyUserName is the type for the username in the context
type contextKeyUserName string

const (
	// UserNameKey is the key for the username in the context
	UserNameKey = contextKeyUserName("username")
)

func GetGinLogger() gin.HandlerFunc {
	return gin.LoggerWithConfig(loggerConfig)
}

// defaultLogFormatter is the default log formatter
func defaultLogFormatter(param gin.LogFormatterParams) string {

	levelSeverity := "INFO"

	if strings.HasPrefix(param.Path, "/internal") {
		levelSeverity = "DEBUG"
	}

	if param.Method == "OPTIONS" {
		levelSeverity = "DEBUG"
	}

	if param.StatusCode >= 500 {
		levelSeverity = "ERROR"
	} else if param.StatusCode >= 400 {
		levelSeverity = "WARN"
	}

	if param.Latency > time.Minute {
		param.Latency = param.Latency.Round(time.Second)
	} else {
		param.Latency = param.Latency.Round(time.Microsecond)
	}

	username := param.Request.Context().Value(UserNameKey)

	name := "<nil>"
	if username != nil {
		name = username.(string)
	}

	if values.V.GinMode == "release" && values.V.JsonLogs {
		return jsonLogFormatter(param, levelSeverity, name)
	} else {
		return consoleLogFormatter(param, levelSeverity, name)
	}
}

func getLevelColor(level string) *color.Color {
	switch level {
	case "DEBUG":
		return color.New(color.BgCyan, color.FgHiWhite)
	case "INFO":
		return color.New(color.BgBlue, color.FgHiWhite)
	case "WARN":
		return color.New(color.BgYellow, color.FgHiBlack)
	case "ERROR":
		return color.New(color.BgRed, color.FgHiWhite)
	case "DPANIC":
		return color.New(color.BgMagenta, color.FgHiWhite)
	case "PANIC":
		return color.New(color.BgMagenta, color.FgHiWhite)
	case "FATAL":
		return color.New(color.BgMagenta, color.FgHiWhite)
	}
	return color.New(color.BgWhite, color.FgHiBlack)
}

func consoleLogFormatter(param gin.LogFormatterParams, levelSeverity string, name string) string {
	var statusColor, methodColor, resetColor string
	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		methodColor = param.MethodColor()
		resetColor = param.ResetColor()
	}

	if len(name) > 10 {
		name = name[:10]
	}

	return fmt.Sprintf("%s %v |%-5s|%s %3d %s| %13s | %10s |%s %-7s %s %#v\n%s",
		colorGin,
		param.TimeStamp.Format("02.01.2006 - 15:04:05"),
		getLevelColor(levelSeverity).Sprintf(" %-5s ", levelSeverity),
		statusColor, param.StatusCode, resetColor,
		param.Latency,
		name,
		methodColor, param.Method, resetColor,
		param.Path,
		param.ErrorMessage,
	)
}
func jsonLogFormatter(param gin.LogFormatterParams, levelSeverity string, name string) string {
	log := make(map[string]interface{})
	log["time"] = param.TimeStamp.Format("02.01.2006 - 15:04:05")
	log["level"] = levelSeverity
	log["status"] = param.StatusCode
	log["latency"] = param.Latency
	log["username"] = name
	log["method"] = param.Method
	log["path"] = param.Path
	log["error"] = param.ErrorMessage
	log["type"] = "gin"
	s, _ := json.Marshal(log)
	return string(s) + "\n"
}
