package log

import (
	"fmt"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/prometheus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"sync"
)

type (
	logConfig struct {
		OnlyConsole     bool
		MaxSize         int  // megabytes
		MaxBackups      int  // 3,
		MaxAge          int  //days
		LocalTime       bool //UTC by default
		ConsoleLogLevel string
		Files           map[string]logFileConfig
	}
	logFileConfig struct {
		Filename string
		LogLevel string
		Encoder  string
	}
)

var (
	sysLogger    *zap.SugaredLogger
	sysRotator   *lumberjack.Logger
	accessLogger *zap.SugaredLogger

	//日志的默认配置
	files = map[string]logFileConfig{
		"system": {
			Filename: "./logs/system.log",
			LogLevel: "debug",
			Encoder:  "console",
		},
		"server": {
			Filename: "./logs/server.log",
			LogLevel: "info",
			Encoder:  "json",
		},
		"trace": {
			Filename: "./logs/trace.log",
			LogLevel: "debug",
			Encoder:  "console",
		},
		"error": {
			Filename: "./logs/error.log",
			LogLevel: "error",
			Encoder:  "console",
		},
	}
	logConf = logConfig{
		OnlyConsole:     true, //若没有配置[log]，则日志只输出到控制台
		MaxSize:         100,
		MaxBackups:      5,
		MaxAge:          30,
		LocalTime:       true,
		ConsoleLogLevel: "debug",
		Files:           files,
	}

	encoderConfig zapcore.EncoderConfig

	logOnce sync.Once
	loading bool
)

func SysLogger() *zap.SugaredLogger {
	logOnce.Do(loadLogConfig)
	return sysLogger
}

func AccessLogger() *zap.SugaredLogger {
	logOnce.Do(loadLogConfig)
	return accessLogger
}

func loadLogConfig() {
	loading = true
	defer func() {
		loading = false
	}()

	if viper.IsSet("log") {
		logConf.OnlyConsole = false //配置了[log]则输出到文件
		if err := viper.UnmarshalKey("log", &logConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		if err := checkLogLevel(logConf.ConsoleLogLevel); err != nil {
			panic(err)
		}
		for file, conf := range logConf.Files {
			if err := checkLogFile(file); err != nil {
				panic(err)
			}
			if err := checkLogLevel(conf.LogLevel); err != nil {
				panic(err)
			}
			if err := checkLogEncoder(conf.Encoder); err != nil {
				panic(err)
			}
		}
	}

	//初始化encoder
	encoderConfig = zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(getZapLogLevel(logConf.ConsoleLogLevel)),
	)

	accessLogger = initAccessLogger(consoleCore,
		zap.Fields(DefaultAccessLogFields()...),
		zap.Hooks(prometheus.ZapHook("access")), //prometheus钩子
		zap.AddCaller(),
	).Sugar()

	sysLogger = initSystemLogger(consoleCore,
		zap.Hooks(prometheus.ZapHook("system")), //prometheus钩子
		zap.AddCaller(),
	).Sugar()
}

//访问日志server.log设置默认字段即可
func DefaultAccessLogFields() []zap.Field {
	if !loading {
		logOnce.Do(loadLogConfig)
	}

	if logConf.OnlyConsole {
		return nil
	}
	var fields []zap.Field
	fields = append(fields, zap.String("ip", env.GetIP()))
	fields = append(fields, zap.String("app", env.GetAppName()))
	return fields
}

func getZapLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func checkLogLevel(level string) error {
	if level == "debug" ||
		level == "info" ||
		level == "error" {
		return nil
	}
	return errors.New("log level illegal")
}

func checkLogFile(file string) error {
	if file != "system" &&
		file != "server" &&
		file != "trace" &&
		file != "error" {
		return errors.New("not supported log file type: should be system|server|trace|error")
	}
	return nil
}

func checkLogEncoder(encoder string) error {
	if encoder == "console" ||
		encoder == "json" {
		return nil
	}
	return errors.New("log encoder illegal: should be console|json")
}

func getLogEncoder(encoder string) zapcore.Encoder {
	switch encoder {
	case "console":
		return zapcore.NewConsoleEncoder(encoderConfig)
	case "json":
		return zapcore.NewJSONEncoder(encoderConfig)
	default:
		panic("log encoder illegal: should be console|json")
	}
}

func getFileLogger(fileConf logFileConfig) (zapcore.Core, *lumberjack.Logger) {
	fileRotator := &lumberjack.Logger{
		Filename:   fileConf.Filename,
		MaxSize:    logConf.MaxSize, // megabytes
		MaxBackups: logConf.MaxBackups,
		MaxAge:     logConf.MaxAge,    //days
		LocalTime:  logConf.LocalTime, //UTC by default
	}

	fileCore := zapcore.NewCore(
		getLogEncoder(fileConf.Encoder),
		zapcore.AddSync(fileRotator),
		zap.NewAtomicLevelAt(getZapLogLevel(fileConf.LogLevel)),
	)
	return fileCore, fileRotator
}

//访问日志初始化
func initAccessLogger(consoleCore zapcore.Core, options ...zap.Option) *zap.Logger {
	var rotators []*lumberjack.Logger
	var cores []zapcore.Core
	if !logConf.OnlyConsole {
		for file, logConf := range logConf.Files {
			if file == "system" {
				continue
			}
			core, rotator := getFileLogger(logConf)
			rotators = append(rotators, rotator)
			cores = append(cores, core)
		}
	}
	cores = append(cores, consoleCore)
	logger := zap.New(zapcore.NewTee(cores...), options...)
	env.AddShutdownFunc(func() error {
		sysLogger.Infow("accessLogger closing...")
		var err error
		_ = logger.Sync()
		for _, rotator := range rotators {
			err2 := rotator.Close()
			if err2 != nil {
				err = err2
			}
		}
		sysLogger.Infow("accessLogger closed")
		return err
	})
	return logger
}

//系统日志初始化
func initSystemLogger(consoleCore zapcore.Core, options ...zap.Option) *zap.Logger {
	var cores []zapcore.Core
	if !logConf.OnlyConsole {
		core, rotator := getFileLogger(logConf.Files["system"])
		sysRotator = rotator
		cores = append(cores, core)
	}
	cores = append(cores, consoleCore)
	logger := zap.New(zapcore.NewTee(cores...), options...)
	return logger
}

func CloseSysLogger() {
	_ = sysLogger.Sync()
	if sysRotator != nil {
		_ = sysRotator.Close()
	}
}
