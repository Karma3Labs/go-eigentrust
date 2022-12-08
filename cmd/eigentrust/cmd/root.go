package cmd

import (
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
	}
	cfgFile string
	logger  zerolog.Logger
)

func Execute() {
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000000000Z07:00"
	logger = zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = os.Stderr
		w.TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"
	})).With().Timestamp().Logger()
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.eigentrust.yaml)")
}
