package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
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
	localhost     bool
	serveCmd      = &cobra.Command{
		Use:   "serve",
		Short: "Serve the EigenTrust API",
		Long:  `Serve the EigenTrust API.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = logger.WithContext(ctx)
			e := echo.New()
			eLogger := lecho.From(logger)
			e.Logger = eLogger
			e.Use(
				middleware.RequestID(),
				middleware.CORS(),
				lecho.Middleware(lecho.Config{Logger: eLogger, NestKey: "req"}),
			)
			server, err := oapiserver.NewStrictServerImpl(ctx)
			if err != nil {
				logger.Err(err).Msg("cannot create server implementation")
				return
			}
			if localhost {
				useFileURI = true
			}
			server.UseFileURI = useFileURI
			openapi.RegisterHandlersWithBaseURL(e,
				openapi.NewStrictHandler(server, nil), "/basic/v1")
			if listenAddress == "" {
				addr := ""
				if localhost {
					addr = "localhost"
				}
				port := 80
				if tls {
					port = 443
				}
				if os.Geteuid() != 0 {
					port += 8000
				}
				listenAddress = fmt.Sprintf("%s:%d", addr, port)
			}
			zerolog.DefaultContextLogger = &logger
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
	serveCmd.PersistentFlags().BoolVarP(&useFileURI, "use-file-uri", "F", false,
		"enable file:// URI based trust matrix/vector loading")
	serveCmd.PersistentFlags().BoolVarP(&localhost, "localhost", "L", false,
		"localhost mode: listen on loopback address and enable file:// URI")
}
