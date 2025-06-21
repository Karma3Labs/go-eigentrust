package convert

import (
	"fmt"

	"k3l.io/go-eigentrust/pkg/api/ogen"
	"k3l.io/go-eigentrust/pkg/api/openapi"
)

// OapiTrustRefToOgen converts oapi-codegen TrustRef to ogen TrustRef
func OapiTrustRefToOgen(oapi *openapi.TrustRef) (*ogen.TrustRef, error) {
	if oapi == nil {
		return nil, nil
	}

	result := &ogen.TrustRef{
		Scheme: ogen.TrustRefScheme(oapi.Scheme),
	}

	switch oapi.Scheme {
	case openapi.Inline:
		inline, err := oapi.AsInlineTrustRef()
		if err != nil {
			return nil, fmt.Errorf("failed to get inline trust ref: %w", err)
		}
		
		ogenInline := ogen.InlineTrustRef{
			Size:    inline.Size,
			Entries: make([]ogen.InlineTrustEntry, len(inline.Entries)),
		}
		
		for i, entry := range inline.Entries {
			ogenEntry := ogen.InlineTrustEntry{V: entry.V}
			
			// IMPORTANT: Check vector entries first, then matrix entries.
			// 
			// This is a workaround for oapi-codegen's permissive type conversion behavior.
			// The oapi-codegen implementation allows AsTrustMatrixEntryIndices() to succeed
			// even for vector entries (returning J=0), which violates OpenAPI spec compliance.
			// 
			// According to the OpenAPI spec:
			// - TrustVectorEntryIndex should only have 'i' field (for vectors)  
			// - TrustMatrixEntryIndices should have both 'i' and 'j' fields (for matrices)
			//
			// By checking vector conversion first, we ensure:
			// - Vector entries are correctly identified and only include 'i' field
			// - Matrix entries are correctly identified and include both 'i' and 'j' fields
			// - The ogen response is fully OpenAPI 3.1 spec compliant
			if vectorEntry, err := entry.AsTrustVectorEntryIndex(); err == nil {
				ogenEntry.OneOf = ogen.InlineTrustEntrySum{
					Type: ogen.TrustVectorEntryIndexInlineTrustEntrySum,
					TrustVectorEntryIndex: ogen.TrustVectorEntryIndex{
						I: vectorEntry.I,
					},
				}
			} else if matrixEntry, err := entry.AsTrustMatrixEntryIndices(); err == nil {
				ogenEntry.OneOf = ogen.InlineTrustEntrySum{
					Type: ogen.TrustMatrixEntryIndicesInlineTrustEntrySum,
					TrustMatrixEntryIndices: ogen.TrustMatrixEntryIndices{
						I: matrixEntry.I,
						J: matrixEntry.J,
					},
				}
			} else {
				return nil, fmt.Errorf("entry %d: invalid entry type", i)
			}
			
			ogenInline.Entries[i] = ogenEntry
		}
		
		result.OneOf = ogen.TrustRefSum{
			Type:           ogen.InlineTrustRefTrustRefSum,
			InlineTrustRef: ogenInline,
		}

	case openapi.Stored:
		stored, err := oapi.AsStoredTrustRef()
		if err != nil {
			return nil, fmt.Errorf("failed to get stored trust ref: %w", err)
		}
		
		result.OneOf = ogen.TrustRefSum{
			Type: ogen.StoredTrustRefTrustRefSum,
			StoredTrustRef: ogen.StoredTrustRef{
				ID: ogen.StoredTrustId(stored.Id),
			},
		}

	case openapi.Objectstorage:
		objStorage, err := oapi.AsObjectStorageTrustRef()
		if err != nil {
			return nil, fmt.Errorf("failed to get object storage trust ref: %w", err)
		}
		
		result.OneOf = ogen.TrustRefSum{
			Type: ogen.ObjectStorageTrustRefTrustRefSum,
			ObjectStorageTrustRef: ogen.ObjectStorageTrustRef{
				URL: objStorage.Url,
			},
		}

	default:
		return nil, fmt.Errorf("unknown scheme: %s", oapi.Scheme)
	}

	return result, nil
}

// OgenTrustRefToOapi converts ogen TrustRef to oapi-codegen TrustRef
func OgenTrustRefToOapi(ogenRef *ogen.TrustRef) (*openapi.TrustRef, error) {
	if ogenRef == nil {
		return nil, nil
	}

	result := &openapi.TrustRef{
		Scheme: openapi.TrustRefScheme(ogenRef.Scheme),
	}

	switch ogenRef.OneOf.Type {
	case ogen.InlineTrustRefTrustRefSum:
		inline := ogenRef.OneOf.InlineTrustRef
		oapiInline := openapi.InlineTrustRef{
			Size:    inline.Size,
			Entries: make([]openapi.InlineTrustEntry, len(inline.Entries)),
		}
		
		for i, entry := range inline.Entries {
			oapiEntry := openapi.InlineTrustEntry{V: entry.V}
			
			switch entry.OneOf.Type {
			case ogen.TrustMatrixEntryIndicesInlineTrustEntrySum:
				matrixEntry := entry.OneOf.TrustMatrixEntryIndices
				err := oapiEntry.FromTrustMatrixEntryIndices(openapi.TrustMatrixEntryIndices{
					I: matrixEntry.I,
					J: matrixEntry.J,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to set matrix entry %d: %w", i, err)
				}
			case ogen.TrustVectorEntryIndexInlineTrustEntrySum:
				vectorEntry := entry.OneOf.TrustVectorEntryIndex
				err := oapiEntry.FromTrustVectorEntryIndex(openapi.TrustVectorEntryIndex{
					I: vectorEntry.I,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to set vector entry %d: %w", i, err)
				}
			default:
				return nil, fmt.Errorf("unknown entry type for entry %d", i)
			}
			
			oapiInline.Entries[i] = oapiEntry
		}
		
		err := result.FromInlineTrustRef(oapiInline)
		if err != nil {
			return nil, fmt.Errorf("failed to set inline trust ref: %w", err)
		}

	case ogen.StoredTrustRefTrustRefSum:
		stored := ogenRef.OneOf.StoredTrustRef
		err := result.FromStoredTrustRef(openapi.StoredTrustRef{
			Id: openapi.StoredTrustId(stored.ID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set stored trust ref: %w", err)
		}

	case ogen.ObjectStorageTrustRefTrustRefSum:
		objStorage := ogenRef.OneOf.ObjectStorageTrustRef
		err := result.FromObjectStorageTrustRef(openapi.ObjectStorageTrustRef{
			Url: objStorage.URL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set object storage trust ref: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown trust ref type")
	}

	return result, nil
}

// OptTrustRefToOapi converts ogen OptTrustRef to oapi-codegen *TrustRef
func OptTrustRefToOapi(opt ogen.OptTrustRef) (*openapi.TrustRef, error) {
	if !opt.Set {
		return nil, nil
	}
	return OgenTrustRefToOapi(&opt.Value)
}

// OapiToOptTrustRef converts oapi-codegen *TrustRef to ogen OptTrustRef
func OapiToOptTrustRef(oapi *openapi.TrustRef) (ogen.OptTrustRef, error) {
	if oapi == nil {
		return ogen.OptTrustRef{Set: false}, nil
	}
	
	converted, err := OapiTrustRefToOgen(oapi)
	if err != nil {
		return ogen.OptTrustRef{}, err
	}
	
	return ogen.OptTrustRef{
		Set:   true,
		Value: *converted,
	}, nil
}