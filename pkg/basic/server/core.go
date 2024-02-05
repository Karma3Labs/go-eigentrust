package server

import "github.com/rs/zerolog"

type Core struct {
	logger           zerolog.Logger
	storedLocalTrust NamedTrustMatrices
}

func NewCore(logger zerolog.Logger) *Core {
	return &Core{
		logger: logger,
	}
}
