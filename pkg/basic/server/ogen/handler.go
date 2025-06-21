package ogenserver

import (
	"context"
	"errors"

	"k3l.io/go-eigentrust/pkg/api/convert"
	"k3l.io/go-eigentrust/pkg/api/ogen"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	"k3l.io/go-eigentrust/pkg/basic/server"
	oapiserver "k3l.io/go-eigentrust/pkg/basic/server/oapi"
)

// Handler implements the ogen.Handler interface by wrapping the existing oapi-codegen implementation
type Handler struct {
	impl *oapiserver.StrictServerImpl
}

// NewHandler creates a new ogen handler that reuses the existing business logic
func NewHandler(ctx context.Context) (*Handler, error) {
	impl, err := oapiserver.NewStrictServerImpl(ctx)
	if err != nil {
		return nil, err
	}

	return &Handler{
		impl: impl,
	}, nil
}

// SetUseFileURI configures whether file:// URIs are allowed
func (h *Handler) SetUseFileURI(useFileURI bool) {
	h.impl.UseFileURI = useFileURI
}

// Compute implements the ogen compute operation by converting types and delegating to existing implementation
func (h *Handler) Compute(ctx context.Context, req *ogen.ComputeRequestBody) (ogen.ComputeRes, error) {
	// Convert ogen request to oapi-codegen format
	oapiReq, err := convert.OgenComputeRequestToOapi(req)
	if err != nil {
		return &ogen.InvalidRequest{
			Message: "Failed to convert request: " + err.Error(),
		}, nil
	}

	// Create oapi-codegen request object
	oapiRequestObj := openapi.ComputeRequestObject{
		Body: oapiReq,
	}

	// Call existing implementation  
	oapiResp, err := h.impl.Compute(ctx, oapiRequestObj)
	if err != nil {
		return nil, err
	}

	// Handle different response types
	switch resp := oapiResp.(type) {
	case openapi.Compute200JSONResponse:
		// The response is a TrustRef directly
		trustRef := openapi.TrustRef(resp.ComputeResponseOKJSONResponse)
		ogenResp, err := convert.OapiComputeResponseToOgen(&trustRef)
		if err != nil {
			return &ogen.InvalidRequest{
				Message: "Failed to convert response: " + err.Error(),
			}, nil
		}
		return ogenResp, nil

	case openapi.Compute400JSONResponse:
		return &ogen.InvalidRequest{
			Message: resp.Message,
		}, nil

	default:
		return &ogen.InvalidRequest{
			Message: "Unknown response type",
		}, nil
	}
}

// ComputeWithStats implements the ogen computeWithStats operation
func (h *Handler) ComputeWithStats(ctx context.Context, req *ogen.ComputeRequestBody) (ogen.ComputeWithStatsRes, error) {
	// Convert ogen request to oapi-codegen format
	oapiReq, err := convert.OgenComputeRequestToOapi(req)
	if err != nil {
		return &ogen.InvalidRequest{
			Message: "Failed to convert request: " + err.Error(),
		}, nil
	}

	// Create oapi-codegen request object
	oapiRequestObj := openapi.ComputeWithStatsRequestObject{
		Body: oapiReq,
	}

	// Call existing implementation
	oapiResp, err := h.impl.ComputeWithStats(ctx, oapiRequestObj)
	if err != nil {
		return nil, err
	}

	// Handle different response types
	switch resp := oapiResp.(type) {
	case openapi.ComputeWithStats200JSONResponse:
		// Convert successful response
		ogenTrustRef, err := convert.OapiComputeResponseToOgen(&resp.EigenTrust)
		if err != nil {
			return &ogen.InvalidRequest{
				Message: "Failed to convert trust ref: " + err.Error(),
			}, nil
		}
		
		ogenStats := convert.OapiFlatTailStatsToOgen(&resp.FlatTailStats)
		
		return &ogen.ComputeWithStatsResponseOK{
			EigenTrust:    *ogenTrustRef,
			FlatTailStats: *ogenStats,
		}, nil

	case openapi.ComputeWithStats400JSONResponse:
		return &ogen.InvalidRequest{
			Message: resp.Message,
		}, nil

	default:
		return &ogen.InvalidRequest{
			Message: "Unknown response type",
		}, nil
	}
}

// DeleteLocalTrust implements the ogen deleteLocalTrust operation
func (h *Handler) DeleteLocalTrust(ctx context.Context, params ogen.DeleteLocalTrustParams) (ogen.DeleteLocalTrustRes, error) {
	oapiRequestObj := openapi.DeleteLocalTrustRequestObject{
		Id: openapi.LocalTrustIdParam(params.ID),
	}

	oapiResp, err := h.impl.DeleteLocalTrust(ctx, oapiRequestObj)
	if err != nil {
		return nil, err
	}

	switch oapiResp.(type) {
	case openapi.DeleteLocalTrust204Response:
		return &ogen.DeleteLocalTrustNoContent{}, nil
	case openapi.DeleteLocalTrust404Response:
		return &ogen.DeleteLocalTrustNotFound{}, nil
	default:
		return &ogen.DeleteLocalTrustNotFound{}, nil
	}
}

// GetLocalTrust implements the ogen getLocalTrust operation
func (h *Handler) GetLocalTrust(ctx context.Context, params ogen.GetLocalTrustParams) (ogen.GetLocalTrustRes, error) {
	oapiRequestObj := openapi.GetLocalTrustRequestObject{
		Id: openapi.LocalTrustIdParam(params.ID),
	}

	oapiResp, err := h.impl.GetLocalTrust(ctx, oapiRequestObj)
	if err != nil {
		return nil, err
	}

	switch resp := oapiResp.(type) {
	case openapi.GetLocalTrust200JSONResponse:
		// Convert the inline trust ref to ogen format
		ogenInline := ogen.InlineTrustRef{
			Size:    resp.LocalTrustGetResponseOKJSONResponse.Size,
			Entries: make([]ogen.InlineTrustEntry, len(resp.LocalTrustGetResponseOKJSONResponse.Entries)),
		}

		for i, entry := range resp.LocalTrustGetResponseOKJSONResponse.Entries {
			ogenEntry := ogen.InlineTrustEntry{V: entry.V}
			
			// Determine entry type and convert
			if matrixEntry, err := entry.AsTrustMatrixEntryIndices(); err == nil {
				ogenEntry.OneOf = ogen.InlineTrustEntrySum{
					Type: ogen.TrustMatrixEntryIndicesInlineTrustEntrySum,
					TrustMatrixEntryIndices: ogen.TrustMatrixEntryIndices{
						I: matrixEntry.I,
						J: matrixEntry.J,
					},
				}
			} else if vectorEntry, err := entry.AsTrustVectorEntryIndex(); err == nil {
				ogenEntry.OneOf = ogen.InlineTrustEntrySum{
					Type: ogen.TrustVectorEntryIndexInlineTrustEntrySum,
					TrustVectorEntryIndex: ogen.TrustVectorEntryIndex{
						I: vectorEntry.I,
					},
				}
			} else {
				return &ogen.GetLocalTrustNotFound{}, nil
			}
			
			ogenInline.Entries[i] = ogenEntry
		}

		return &ogenInline, nil

	case openapi.GetLocalTrust404Response:
		return &ogen.GetLocalTrustNotFound{}, nil

	default:
		return &ogen.GetLocalTrustNotFound{}, nil
	}
}

// GetStatus implements the ogen getStatus operation
func (h *Handler) GetStatus(ctx context.Context) (ogen.GetStatusRes, error) {
	oapiRequestObj := openapi.GetStatusRequestObject{}

	oapiResp, err := h.impl.GetStatus(ctx, oapiRequestObj)
	if err != nil {
		return &ogen.GetStatusInternalServerError{
			Message: "Internal server error",
		}, nil
	}

	switch resp := oapiResp.(type) {
	case openapi.GetStatus200JSONResponse:
		return &ogen.GetStatusOK{
			Message: resp.ServerReadyJSONResponse.Message,
		}, nil
	default:
		return &ogen.GetStatusInternalServerError{
			Message: "Server not ready",
		}, nil
	}
}

// HeadLocalTrust implements the ogen headLocalTrust operation
func (h *Handler) HeadLocalTrust(ctx context.Context, params ogen.HeadLocalTrustParams) (ogen.HeadLocalTrustRes, error) {
	oapiRequestObj := openapi.HeadLocalTrustRequestObject{
		Id: openapi.LocalTrustIdParam(params.ID),
	}

	oapiResp, err := h.impl.HeadLocalTrust(ctx, oapiRequestObj)
	if err != nil {
		return nil, err
	}

	switch oapiResp.(type) {
	case openapi.HeadLocalTrust204Response:
		return &ogen.HeadLocalTrustNoContent{}, nil
	case openapi.HeadLocalTrust404Response:
		return &ogen.HeadLocalTrustNotFound{}, nil
	default:
		return &ogen.HeadLocalTrustNotFound{}, nil
	}
}

// UpdateLocalTrust implements the ogen updateLocalTrust operation
func (h *Handler) UpdateLocalTrust(ctx context.Context, req *ogen.TrustRef, params ogen.UpdateLocalTrustParams) (ogen.UpdateLocalTrustRes, error) {
	// Convert ogen TrustRef to oapi-codegen format
	oapiTrustRef, err := convert.OgenTrustRefToOapi(req)
	if err != nil {
		return &ogen.InvalidRequest{
			Message: "Failed to convert request: " + err.Error(),
		}, nil
	}

	oapiParams := openapi.UpdateLocalTrustParams{}
	if params.Merge.Set {
		merge := params.Merge.Value
		oapiParams.Merge = &merge
	}

	oapiRequestObj := openapi.UpdateLocalTrustRequestObject{
		Id:     openapi.LocalTrustIdParam(params.ID),
		Body:   oapiTrustRef,
		Params: oapiParams,
	}

	oapiResp, err := h.impl.UpdateLocalTrust(ctx, oapiRequestObj)
	if err != nil {
		var httpError server.HTTPError
		if errors.As(err, &httpError) && httpError.Code == 400 {
			return &ogen.InvalidRequest{
				Message: httpError.Inner.Error(),
			}, nil
		}
		return nil, err
	}

	switch oapiResp.(type) {
	case openapi.UpdateLocalTrust200Response:
		return &ogen.UpdateLocalTrustOK{}, nil
	case openapi.UpdateLocalTrust201Response:
		return &ogen.UpdateLocalTrustCreated{}, nil
	default:
		return &ogen.InvalidRequest{
			Message: "Unknown response type",
		}, nil
	}
}