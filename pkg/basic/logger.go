package basic

import "github.com/rs/zerolog"

var logger = zerolog.New(zerolog.NewConsoleWriter())

func Logger() zerolog.Logger             { return logger }
func SetLogger(newLogger zerolog.Logger) { logger = newLogger }
