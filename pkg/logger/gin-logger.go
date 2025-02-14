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

var (
	logUsername    = true
	templateString = ""
)

func GetGinLogger(logUsernameParam bool) gin.HandlerFunc {
	logUsername = logUsernameParam

	stringTemplate := "%s %v |%-5s|%s %3d %s| %13s |"
	if logUsername {
		stringTemplate += " %10s |"
	}
	stringTemplate += "%s %-7s %s %#v\n%s"

	templateString = stringTemplate

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

	name := "<nil>"

	if logUsername {

		username := param.Request.Context().Value(UserNameKey)

		if username != nil {
			name = username.(string)
		}
	}

	if values.V.JsonLogs {
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
	// array with any
	loggingArgs := []any{}
	loggingArgs = append(loggingArgs, colorGin)
	loggingArgs = append(loggingArgs, param.TimeStamp.Format("02.01.2006 - 15:04:05"))
	loggingArgs = append(loggingArgs, getLevelColor(levelSeverity).Sprintf(" %-5s ", levelSeverity))
	loggingArgs = append(loggingArgs, statusColor)
	loggingArgs = append(loggingArgs, param.StatusCode)
	loggingArgs = append(loggingArgs, resetColor)
	loggingArgs = append(loggingArgs, param.Latency)
	if logUsername {
		loggingArgs = append(loggingArgs, name)
	}
	loggingArgs = append(loggingArgs, methodColor)
	loggingArgs = append(loggingArgs, param.Method)
	loggingArgs = append(loggingArgs, resetColor)
	loggingArgs = append(loggingArgs, param.Path)
	loggingArgs = append(loggingArgs, param.ErrorMessage)

	return fmt.Sprintf(templateString, loggingArgs...)
}
func jsonLogFormatter(param gin.LogFormatterParams, levelSeverity string, name string) string {
	log := make(map[string]interface{})
	log["time"] = param.TimeStamp.Format("02.01.2006 - 15:04:05")
	log["level"] = levelSeverity
	log["status"] = param.StatusCode
	log["latency"] = param.Latency
	if logUsername {
		log["username"] = name
	}
	log["method"] = param.Method
	log["path"] = param.Path
	log["error"] = param.ErrorMessage
	log["type"] = "gin"
	s, _ := json.Marshal(log)
	return string(s) + "\n"
}
