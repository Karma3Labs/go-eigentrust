package server

import (
	"math/big"

	"github.com/rs/zerolog"
)

type ComputeParams struct {
	localTrustId  string
	preTrustId    string
	alpha         *float64
	epsilon       *float64
	globalTrustId string
	maxIterations *int
}

type JobSpec struct {
	computeParams ComputeParams

	// period is the re-computation period.  nil if one-shot job.
	period *big.Int
	// TODO(ek): Reinstate upload schemes
}

type PeriodicJob struct {
}

type Core struct {
	Logger              zerolog.Logger
	StoredTrustMatrices NamedTrustMatrices
	StoredTrustVectors  NamedTrustVectors
}

func NewCore(logger zerolog.Logger) *Core {
	return &Core{
		Logger: logger,
	}
}
