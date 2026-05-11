package utils

import (
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// Verbose enables diagnostic logging across the manager.
var Verbose bool

var (
	loggerMu sync.RWMutex
	logger   = log.NewWithOptions(os.Stderr, log.Options{
		Level:           log.InfoLevel,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
	})
)

// ConfigureLogging updates the process-global logger settings.
// Call this early (from CLI) and whenever flags change.
func ConfigureLogging(verbose bool) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	Verbose = verbose
	if verbose {
		logger.SetLevel(log.DebugLevel)
		logger.SetReportCaller(true)
	} else {
		logger.SetLevel(log.InfoLevel)
		logger.SetReportCaller(false)
	}

	// Always include timestamps for operational runs.
	logger.SetReportTimestamp(true)
	logger.SetTimeFormat(time.RFC3339)
}

// L returns the shared logger instance. Prefer the package helpers (`Info`, `Warn`, ...)
// unless you need advanced APIs like `.With(...)`.
func L() *log.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return logger
}

// Logf is retained for backwards compatibility with existing diagnostic logging.
// It maps to Debugf so it only shows up with `--verbose` (or when the level is debug).
func Logf(format string, args ...any) { L().Debugf(format, args...) }

func Debug(msg any, keyvals ...any) { L().Debug(msg, keyvals...) }
func Info(msg any, keyvals ...any)  { L().Info(msg, keyvals...) }
func Warn(msg any, keyvals ...any)  { L().Warn(msg, keyvals...) }
func Error(msg any, keyvals ...any) { L().Error(msg, keyvals...) }
