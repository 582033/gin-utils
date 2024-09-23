package log

import (
	"fmt"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/log/rotate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"
)

var log *zap.SugaredLogger

var defaultLevel int = -1

var logLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
	return lvl >= zapcore.Level(config.Get("log.level").Int(defaultLevel))
})

type Level int8

const (
	DebugLevel Level = iota - 1
	InfoLevel
	WarnLevel
	ErrorLevel
	DPanicLevel
	PanicLevel
	FatalLevel
)

func init() {

	_ = config.LoadEnv("CORE_LOG_")
	filePath := getFilePath()
	file := zapcore.AddSync(&rotate.Logger{
		Filename:  filePath,
		MaxSize:   config.Get("core.log.size").Int(1024), //MB
		LocalTime: true,
		Compress:  config.Get("core.log.compress").Bool(true),
		MaxAge:    config.Get("core.log.age").Int(64),
	})

	var allCore []zapcore.Core

	feiShuEncoderConfig := zapcore.EncoderConfig{
		TimeKey:    "ts",
		MessageKey: "msg",
		EncodeTime: zapcore.ISO8601TimeEncoder,
	}

	feiShuEncoder := zapcore.NewJSONEncoder(feiShuEncoderConfig)
	feiShuWriter := NewFeiShuNoticeCore(feiShuEncoder)

	//写文件
	productionEncoderConfig := zap.NewProductionEncoderConfig()
	productionEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(productionEncoderConfig)
	fileWriter := zapcore.NewCore(
		fileEncoder,
		file,
		logLevel,
	)

	//写控制台
	consoleDebugging := zapcore.Lock(os.Stdout)
	developmentEncoderConfig := zap.NewDevelopmentEncoderConfig()
	developmentEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(developmentEncoderConfig)

	consoleWriter := zapcore.NewCore(
		consoleEncoder,
		consoleDebugging,
		zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return logLevel.Enabled(lvl) &&
				(lvl >= zapcore.ErrorLevel ||
					zapcore.Level(config.Get("log.level").Int(defaultLevel)) == zapcore.DebugLevel)
		}))

	allCore = append(allCore, fileWriter, consoleWriter, feiShuWriter)

	core := zapcore.NewTee(allCore...).With([]zap.Field{
		//zap.String("app", "appName"),
	})

	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	log = logger.Sugar()
	fmt.Printf("log file: %s\n", filePath)
}

func getFilePath() string {
	_ = config.LoadEnv("POD_IP_")
	localIp := config.Get("pod.ip.address").String("")
	filepathSeparator := string(filepath.Separator)
	if localIp != "" {
		//dev -> 项目名 -> 服务名 -> 获取本地IP-> 日志文件.log
		_ = config.LoadEnv("APP_")
		return fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s", config.Get("core.log.path").String(os.TempDir()),
			filepathSeparator,
			config.Get("app.env").String(""),
			filepathSeparator,
			config.Get("app.project.name").String(""),
			filepathSeparator,
			getAppName(),
			filepathSeparator,
			localIp,
			filepathSeparator,
			config.Get("core.log.filename").String(getAppName()+".log"))
	}

	logfile := config.Get("core.log.path").String(os.TempDir()) +
		string(filepath.Separator) +
		config.Get("core.log.filename").String(getAppName()+".log")
	return logfile
}

func GetLevel() string {
	return zapcore.Level(config.Get("log.level").Int(defaultLevel)).String()
}

func LoggerLevel() zapcore.Level {
	return zapcore.Level(config.Get("log.level").Int(defaultLevel))
}

func getAppName() string {
	full := os.Args[0]
	return filepath.Base(full)
}

func Logger() *zap.SugaredLogger {
	return log
}

func Debug(args ...interface{}) {
	log.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	log.Debugf(template, args...)
}

func Println(args ...interface{}) {
	log.Info(args...)
}

func Printf(template string, args ...interface{}) {
	log.Infof(template, args...)
}

func Info(args ...interface{}) {
	log.Info(args...)
}

func Infof(template string, args ...interface{}) {
	log.Infof(template, args...)
}

func Warn(args ...interface{}) {
	log.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	log.Warnf(template, args...)
}

func Error(args ...interface{}) {
	log.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	log.Errorf(template, args...)
}

func Panic(args ...interface{}) {
	log.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	log.Panicf(template, args...)
}

func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	log.Fatalf(template, args...)
}
