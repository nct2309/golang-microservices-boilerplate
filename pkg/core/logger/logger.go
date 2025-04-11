package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel defines the level of logging
type LogLevel string

const (
	// LogLevelDebug represents debug level logging
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo represents info level logging
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn represents warning level logging
	LogLevelWarn LogLevel = "warn"
	// LogLevelError represents error level logging
	LogLevelError LogLevel = "error"
	// LogLevelFatal represents fatal level logging
	LogLevelFatal LogLevel = "fatal"
)

// LogConfig contains configuration for the logger
type LogConfig struct {
	Level      LogLevel
	Format     string // json or console
	OutputPath string // stdout, stderr, or a file path
	AppName    string
	AppEnv     string
	FileConfig *LogFileConfig
}

// LogFileConfig contains configuration for file logging
type LogFileConfig struct {
	MaxSize    int  // Maximum size in megabytes
	MaxBackups int  // Maximum number of backups
	MaxAge     int  // Maximum days to retain logs
	Compress   bool // Whether to compress rotated logs
}

// DefaultLogConfig returns a default configuration for the logger
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      LogLevelInfo,
		Format:     "console",
		OutputPath: "stdout",
		AppName:    "service",
		AppEnv:     "development",
		FileConfig: &LogFileConfig{
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
	}
}

// LoadLogConfigFromEnv loads logger configuration from environment variables
func LoadLogConfigFromEnv() *LogConfig {
	config := DefaultLogConfig()

	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Level = LogLevel(strings.ToLower(level))
	}

	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = strings.ToLower(format)
	}

	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		config.OutputPath = output
	}

	if name := os.Getenv("APP_NAME"); name != "" {
		config.AppName = name
	} else if name := os.Getenv("SERVER_APP_NAME"); name != "" {
		config.AppName = name
	}

	if env := os.Getenv("APP_ENV"); env != "" {
		config.AppEnv = env
	}

	// File logging settings
	if maxSizeStr := os.Getenv("LOG_FILE_MAX_SIZE"); maxSizeStr != "" {
		if maxSize, err := fmt.Sscanf(maxSizeStr, "%d", &config.FileConfig.MaxSize); err != nil || maxSize <= 0 {
			config.FileConfig.MaxSize = 100
		}
	}

	if maxBackupsStr := os.Getenv("LOG_FILE_MAX_BACKUPS"); maxBackupsStr != "" {
		if maxBackups, err := fmt.Sscanf(maxBackupsStr, "%d", &config.FileConfig.MaxBackups); err != nil || maxBackups < 0 {
			config.FileConfig.MaxBackups = 3
		}
	}

	if maxAgeStr := os.Getenv("LOG_FILE_MAX_AGE"); maxAgeStr != "" {
		if maxAge, err := fmt.Sscanf(maxAgeStr, "%d", &config.FileConfig.MaxAge); err != nil || maxAge < 0 {
			config.FileConfig.MaxAge = 28
		}
	}

	if compressStr := os.Getenv("LOG_FILE_COMPRESS"); compressStr != "" {
		config.FileConfig.Compress = compressStr == "true" || compressStr == "1"
	}

	return config
}

// Logger defines the interface for logging operations
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Fatal(msg string, args ...interface{})
	With(args ...interface{}) Logger
	Named(name string) Logger
}

// ZapLogger implements the Logger interface using zap
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewLogger creates a new logger with the specified configuration
func NewLogger(config *LogConfig) (Logger, error) {
	var zapLogger *zap.Logger

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format(time.RFC3339))
	}

	// Determine log level
	var level zapcore.Level
	switch config.Level {
	case LogLevelDebug:
		level = zapcore.DebugLevel
	case LogLevelInfo:
		level = zapcore.InfoLevel
	case LogLevelWarn:
		level = zapcore.WarnLevel
	case LogLevelError:
		level = zapcore.ErrorLevel
	case LogLevelFatal:
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	// Create a level enabler
	levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level
	})

	// Setup output
	var cores []zapcore.Core

	// Add console output if needed
	if config.Format == "console" || config.OutputPath == "stdout" || config.OutputPath == "stderr" {
		// Console encoder
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

		// Console output
		var consoleOutput zapcore.WriteSyncer
		if config.OutputPath == "stderr" {
			consoleOutput = zapcore.AddSync(os.Stderr)
		} else {
			consoleOutput = zapcore.AddSync(os.Stdout)
		}

		// Create console core
		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleOutput, levelEnabler))
	}

	// Add file output if needed
	if config.OutputPath != "stdout" && config.OutputPath != "stderr" && config.OutputPath != "" {
		// JSON encoder for files
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

		// File output with rotation
		fileOutput := zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.OutputPath,
			MaxSize:    config.FileConfig.MaxSize,
			MaxBackups: config.FileConfig.MaxBackups,
			MaxAge:     config.FileConfig.MaxAge,
			Compress:   config.FileConfig.Compress,
		})

		// Create file core
		cores = append(cores, zapcore.NewCore(fileEncoder, fileOutput, levelEnabler))
	}

	// Create a tee with all cores
	core := zapcore.NewTee(cores...)

	// Create logger with the tee
	zapLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	// Add default fields
	zapLogger = zapLogger.With(
		zap.String("service", config.AppName),
		zap.String("environment", config.AppEnv),
	)

	return &ZapLogger{logger: zapLogger.Sugar()}, nil
}

// NewLoggerFromEnv creates a new logger with configuration from environment
func NewLoggerFromEnv() (Logger, error) {
	return NewLogger(LoadLogConfigFromEnv())
}

// Debug logs a message at debug level
func (l *ZapLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debugw(msg, args...)
}

// Info logs a message at info level
func (l *ZapLogger) Info(msg string, args ...interface{}) {
	l.logger.Infow(msg, args...)
}

// Warn logs a message at warning level
func (l *ZapLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warnw(msg, args...)
}

// Error logs a message at error level
func (l *ZapLogger) Error(msg string, args ...interface{}) {
	l.logger.Errorw(msg, args...)
}

// Fatal logs a message at fatal level then exits
func (l *ZapLogger) Fatal(msg string, args ...interface{}) {
	l.logger.Fatalw(msg, args...)
}

// With adds context fields to the logger
func (l *ZapLogger) With(args ...interface{}) Logger {
	return &ZapLogger{logger: l.logger.With(args...)}
}

// Named adds a sub-scope to the logger
func (l *ZapLogger) Named(name string) Logger {
	return &ZapLogger{logger: l.logger.Named(name)}
}
