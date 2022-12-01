package main

import (
	"flag"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/basic"
)

var (
	listenAddress = flag.String("listen_address", ":8080",
		"server listen address to bind to")
	logger zerolog.Logger
)

func main() {
	flag.Parse()
	logger = zerolog.New(os.Stderr)
	basic.SetLogger(logger)
	e := echo.New()
	server := basic.StrictServerImpl{}
	basic.RegisterHandlers(e, basic.NewStrictHandler(&server, nil))
	err := e.Start(*listenAddress)
	if err != nil {
		logger.Err(err).Msg("server did not shut down gracefully")
	}
}
