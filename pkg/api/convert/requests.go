package convert

import (
	"k3l.io/go-eigentrust/pkg/api/ogen"
	"k3l.io/go-eigentrust/pkg/api/openapi"
)

// OgenComputeRequestToOapi converts ogen compute request to oapi-codegen format
func OgenComputeRequestToOapi(req *ogen.ComputeRequestBody) (*openapi.ComputeRequestBody, error) {
	if req == nil {
		return nil, nil
	}

	// Convert required LocalTrust
	localTrust, err := OgenTrustRefToOapi(&req.LocalTrust)
	if err != nil {
		return nil, err
	}

	result := &openapi.ComputeRequestBody{
		LocalTrust: *localTrust,
	}

	// Convert optional fields
	if req.InitialTrust.Set {
		initialTrust, err := OptTrustRefToOapi(req.InitialTrust)
		if err != nil {
			return nil, err
		}
		result.InitialTrust = initialTrust
	}

	if req.PreTrust.Set {
		preTrust, err := OptTrustRefToOapi(req.PreTrust)
		if err != nil {
			return nil, err
		}
		result.PreTrust = preTrust
	}

	// Convert optional numeric fields
	if req.Alpha.Set {
		alpha := req.Alpha.Value
		result.Alpha = &alpha
	}

	if req.Epsilon.Set {
		epsilon := req.Epsilon.Value
		result.Epsilon = &epsilon
	}

	if req.FlatTail.Set {
		flatTail := req.FlatTail.Value
		result.FlatTail = &flatTail
	}

	if req.NumLeaders.Set {
		numLeaders := req.NumLeaders.Value
		result.NumLeaders = &numLeaders
	}

	if req.MaxIterations.Set {
		maxIterations := req.MaxIterations.Value
		result.MaxIterations = &maxIterations
	}

	if req.MinIterations.Set {
		minIterations := req.MinIterations.Value
		result.MinIterations = &minIterations
	}

	if req.CheckFreq.Set {
		checkFreq := req.CheckFreq.Value
		result.CheckFreq = &checkFreq
	}

	// Convert output destination fields
	if req.GlobalTrust.Set {
		globalTrust, err := OptTrustRefToOapi(req.GlobalTrust)
		if err != nil {
			return nil, err
		}
		result.GlobalTrust = globalTrust
	}

	if req.EffectiveLocalTrust.Set {
		effectiveLocalTrust, err := OptTrustRefToOapi(req.EffectiveLocalTrust)
		if err != nil {
			return nil, err
		}
		result.EffectiveLocalTrust = effectiveLocalTrust
	}

	if req.EffectivePreTrust.Set {
		effectivePreTrust, err := OptTrustRefToOapi(req.EffectivePreTrust)
		if err != nil {
			return nil, err
		}
		result.EffectivePreTrust = effectivePreTrust
	}

	if req.EffectiveInitialTrust.Set {
		effectiveInitialTrust, err := OptTrustRefToOapi(req.EffectiveInitialTrust)
		if err != nil {
			return nil, err
		}
		result.EffectiveInitialTrust = effectiveInitialTrust
	}

	return result, nil
}

// OapiComputeResponseToOgen converts oapi-codegen compute response to ogen format
func OapiComputeResponseToOgen(resp *openapi.TrustRef) (*ogen.TrustRef, error) {
	return OapiTrustRefToOgen(resp)
}

// OapiFlatTailStatsToOgen converts oapi-codegen FlatTailStats to ogen format
func OapiFlatTailStatsToOgen(stats *openapi.FlatTailStats) *ogen.FlatTailStats {
	if stats == nil {
		return nil
	}

	return &ogen.FlatTailStats{
		Length:    stats.Length,
		Threshold: stats.Threshold,
		DeltaNorm: stats.DeltaNorm,
		Ranking:   stats.Ranking,
	}
}