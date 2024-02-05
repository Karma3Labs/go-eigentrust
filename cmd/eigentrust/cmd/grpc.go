package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	computepb "k3l.io/go-eigentrust/pkg/api/pb/compute"
	trustmatrixpb "k3l.io/go-eigentrust/pkg/api/pb/trustmatrix"
	trustvectorpb "k3l.io/go-eigentrust/pkg/api/pb/trustvector"
	"k3l.io/go-eigentrust/pkg/basic/server"
	grpcserver "k3l.io/go-eigentrust/pkg/basic/server/grpc"
)

var (
	grpcCmd = &cobra.Command{
		Use:   "grpc",
		Short: "Serve the EigenTrust API over gRPC",
		Long:  `Serve the EigenTrust API over gRPC.`,
		Run: func(cmd *cobra.Command, args []string) {
			//e := echo.New()
			//eLogger := lecho.From(logger)
			//e.Logger = eLogger
			//e.Use(
			//	//middleware.RequestID(),
			//	//middleware.CORS(),
			//	lecho.Middleware(lecho.Config{Logger: eLogger, NestKey: "req"}),
			//)
			//server := basic.NewEchoStrictServerImpl(logger)
			//basic.RegisterHandlersWithBaseURL(e,
			//	basic.NewStrictHandler(server, nil), "/basic/v1")
			//var err error
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
			//if tls {
			//	err = e.StartTLS(listenAddress, certPathname, keyPathname)
			//} else {
			//	err = e.Start(listenAddress)
			//}
			// TODO(ek): Return nonzero status upon error
			listener, err := net.Listen("tcp", listenAddress)
			if err != nil {
				logger.Err(err).Msg("cannot create listener")
				return
			}
			var opts []grpc.ServerOption
			if tls {
				creds, err := credentials.NewServerTLSFromFile(certPathname,
					keyPathname)
				if err != nil {
					logger.Err(err).Msg("cannot create TLS server")
					return
				}
				opts = append(opts, grpc.Creds(creds))
			}
			core := server.NewCore(logger)
			matrixServer := grpcserver.NewTrustMatrixServer(&core.StoredTrustMatrices)
			vectorServer := grpcserver.NewTrustVectorServer(&core.StoredTrustVectors)
			computeServer := grpcserver.NewGrpcServer(core)

			svr := grpc.NewServer(opts...)
			computepb.RegisterServiceServer(svr, computeServer)
			trustmatrixpb.RegisterServiceServer(svr, matrixServer)
			trustvectorpb.RegisterServiceServer(svr, vectorServer)

			err = svr.Serve(listener)
			if err != nil {
				logger.Err(err).Msg("server did not start or shut down gracefully")
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(grpcCmd)
	// See serve.go for listenAddress, tls, certPathname, keyPathname
}
