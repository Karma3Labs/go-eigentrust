package util

import (
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
