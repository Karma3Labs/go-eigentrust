package util

import (
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

type TimeLogger struct {
	Timer    func() time.Time
	Logger   zerolog.Logger
	LogStart time.Time
	LapStart time.Time
}

func NewTimeLogger(timer func() time.Time, logger zerolog.Logger) *TimeLogger {
	now := timer()
	logger.Trace().Time("now", now).Msg("TimeLogger started")
	return &TimeLogger{
		Timer:    timer,
		Logger:   logger,
		LogStart: now,
		LapStart: now,
	}
}

func NewWallTimeLogger(logger zerolog.Logger) *TimeLogger {
	return NewTimeLogger(time.Now, logger)
}

func (p *TimeLogger) Log(name string) {
	now := time.Now()
	lapTime := now.Sub(p.LapStart)
	cumulative := now.Sub(p.LogStart)
	p.Logger.Trace().
		Str("lap", name).
		Time("now", now).
		Dur("lapTime", lapTime).
		Dur("cumulative", cumulative).
		Msg("finished lap")
	p.LapStart = now
}

// LoggerWithCallerAtDepth recovers the caller at the given stack depth
// and adds it to the given logger under the "func", "file", and "line" keys.
// depth is 0 for the caller of LoggerWithCallerAtDepth.
// It returns the logger unmodified if the given stack frame doesn't exist.
func LoggerWithCallerAtDepth(depth int, logger zerolog.Logger) zerolog.Logger {
	pc, file, line, ok := runtime.Caller(depth + 1)
	if !ok {
		return logger
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return logger
	}
	return logger.With().
		Str("file", file).
		Int("line", line).
		Str("func", fn.Name()).
		Logger()
}

// LoggerWithCaller recovers the calling function of LoggerWithCaller
// and adds it to the given logger under the "func", "file", and "line" keys.
func LoggerWithCaller(logger zerolog.Logger) zerolog.Logger {
	return LoggerWithCallerAtDepth(1, logger)
}

// AddCallerToLogger adds the calling function to the logger
// under the "func", "file", and "line" keys.
func AddCallerToLogger(logger zerolog.Logger) {
	logger = LoggerWithCallerAtDepth(1, logger)
}
