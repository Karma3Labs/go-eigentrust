package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
	"github.com/ziflex/lecho/v3"
	"k3l.io/go-eigentrust/pkg/basic"
)

var (
	listenAddress string
	serveCmd      = &cobra.Command{
		Use:   "serve",
		Short: "Serve the EigenTrust API",
		Long:  `Serve the EigenTrust API.`,
		Run: func(cmd *cobra.Command, args []string) {
			e := echo.New()
			eLogger := lecho.From(logger)
			e.Logger = eLogger
			e.Use(
				middleware.RequestID(),
				lecho.Middleware(lecho.Config{Logger: eLogger, NestKey: "req"}),
			)
			server := basic.StrictServerImpl{Logger: logger}
			basic.RegisterHandlersWithBaseURL(e,
				basic.NewStrictHandler(&server, nil), "/basic/v1")
			logger.Info().Str("listenAddress", listenAddress).Msg("serving")
			err := e.Start(listenAddress)
			if err != nil {
				logger.Err(err).Msg("server did not shut down gracefully")
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVar(&listenAddress, "listen_address",
		":8080", "server listen address to bind to")
}
