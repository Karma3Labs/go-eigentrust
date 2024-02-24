package cmd

import (
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
	"github.com/ziflex/lecho/v3"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	oapiserver "k3l.io/go-eigentrust/pkg/basic/server/oapi"
)

var (
	listenAddress string
	tls           bool
	certPathname  string
	keyPathname   string
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
				middleware.CORS(),
				lecho.Middleware(lecho.Config{Logger: eLogger, NestKey: "req"}),
			)
			server := oapiserver.NewStrictServerImpl(logger)
			openapi.RegisterHandlersWithBaseURL(e,
				openapi.NewStrictHandler(server, nil), "/basic/v1")
			var err error
			if listenAddress == "" {
				port := 80
				if tls {
					port = 443
				}
				if os.Geteuid() != 0 {
					port += 8000
				}
				listenAddress = fmt.Sprintf(":%d", port)
			}
			if tls {
				err = e.StartTLS(listenAddress, certPathname, keyPathname)
			} else {
				err = e.Start(listenAddress)
			}
			if err != nil {
				logger.Err(err).Msg("server did not start or shut down gracefully")
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringVar(&listenAddress, "listen_address",
		"", `server listen address to bind to
(default: automatically choose based upon --tls and effective user ID)`)
	serveCmd.PersistentFlags().BoolVar(&tls, "tls", false, "serve over TLS")
	serveCmd.PersistentFlags().StringVar(&certPathname, "tls-cert",
		"server.crt",
		"TLS server certificate pathname")
	serveCmd.PersistentFlags().StringVar(&keyPathname, "tls-key", "server.key",
		"TLS server private key pathname")
}
