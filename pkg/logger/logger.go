package logger

import (
	"fmt"
	"time"

	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Log variable is a globally accessible variable which will be initialized when the InitializeZapCustomLogger function is executed successfully.
	Log      *zap.Logger
	colorApp = color.New(color.BgHiCyan, color.FgHiWhite).Sprint("[APP]")
)

var logLevelSeverity = map[zapcore.Level]string{
	zapcore.DebugLevel:  "DEBUG",
	zapcore.InfoLevel:   "INFO",
	zapcore.WarnLevel:   "WARN",
	zapcore.ErrorLevel:  "ERROR",
	zapcore.DPanicLevel: "DPANIC",
	zapcore.PanicLevel:  "PANIC",
	zapcore.FatalLevel:  "FATAL",
}

func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {

	levelColor := color.New(color.BgWhite, color.FgHiBlack)

	switch logLevelSeverity[level] {
	case "DEBUG":
		levelColor = color.New(color.BgCyan, color.FgHiWhite)
	case "INFO":
		levelColor = color.New(color.BgBlue, color.FgHiWhite)
	case "WARN":
		levelColor = color.New(color.BgYellow, color.FgHiBlack)
	case "ERROR":
		levelColor = color.New(color.BgRed, color.FgHiWhite)
	case "DPANIC":
		levelColor = color.New(color.BgMagenta, color.FgHiWhite)
	case "PANIC":
		levelColor = color.New(color.BgMagenta, color.FgHiWhite)
	case "FATAL":
		levelColor = color.New(color.BgMagenta, color.FgHiWhite)
	}

	enc.AppendString(colorApp + " " + time.Now().Format("02.01.2006 - 15:04:05") + " |" + levelColor.Sprintf(" %-5s ", logLevelSeverity[level]) + "|")
}
func customLevelEncoderRelease(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {

	enc.AppendString(logLevelSeverity[level])
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("02.01.2006 - 15:04:05"))
}

// InitLogger initializes the logger with the given configuration.
func InitLogger() {
	loglevel := zapcore.DebugLevel

	var conf = zap.Config{}

	if values.V.JsonLogs {
		conf = zap.Config{
			Encoding:         "json",
			Level:            zap.NewAtomicLevelAt(loglevel),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig: zapcore.EncoderConfig{
				LevelKey:     "level",
				MessageKey:   "msg",
				CallerKey:    "file",
				TimeKey:      "time",
				EncodeLevel:  customLevelEncoderRelease,
				EncodeTime:   customTimeEncoder,
				EncodeCaller: zapcore.ShortCallerEncoder,
			},
			InitialFields: map[string]interface{}{
				"type": "app",
			},
		}
	} else {
		conf = zap.Config{
			Encoding:         "console",
			Level:            zap.NewAtomicLevelAt(loglevel),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			EncoderConfig: zapcore.EncoderConfig{
				LevelKey:     "level",
				MessageKey:   "msg",
				CallerKey:    "file",
				EncodeLevel:  customLevelEncoder,
				EncodeTime:   customTimeEncoder,
				EncodeCaller: zapcore.ShortCallerEncoder,
			},
		}
	}

	foo, err := conf.Build()
	if err != nil {
		fmt.Println("Error while initializing logger: ", err)
		return
	}
	Log = foo
}
