package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "eigentrust",
		Short: "EigenTrust CLI",
		Long: `EigenTrust CLI provides an EigenTrust server
as well as a client to interact with the server.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var logWriter io.Writer
			switch logFile {
			case "-":
				logWriter = os.Stdout
			case "":
				logWriter = zerolog.NewConsoleWriter(
					func(w *zerolog.ConsoleWriter) {
						w.Out = os.Stderr
						w.TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"
					})
			default:
				w, err := os.OpenFile(logFile,
					os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o0777)
				if err != nil {
					return fmt.Errorf("cannot open log file: %w", err)
				}
				logWriter = w
			}
			logger = zerolog.New(logWriter).With().Timestamp().Logger()
			zerolog.DefaultContextLogger = &logger
			return nil
		},
	}
	cfgFile string
	logFile string
	logger  zerolog.Logger

	useFileURI bool
)

func Execute() {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000000000Z07:00"
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.eigentrust.yaml)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "",
		"log file (- means stdout; default: colorized stderr)")
}
