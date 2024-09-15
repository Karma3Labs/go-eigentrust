package server

import "C"

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mohae/deepcopy"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/api/openapi"
	"k3l.io/go-eigentrust/pkg/basic"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type OAPIStrictServerImpl struct {
	core       *Core
	UseFileURI bool
}

func NewOAPIStrictServerImpl(ctx context.Context) (
	*OAPIStrictServerImpl, error,
) {
	core, err := NewCore(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create server core: %w", err)
	}
	return &OAPIStrictServerImpl{
		core:       core,
		UseFileURI: false,
	}, nil
}

func (server *OAPIStrictServerImpl) GetStatus(
	context.Context, openapi.GetStatusRequestObject,
) (openapi.GetStatusResponseObject, error) {
	return openapi.GetStatus200JSONResponse{
		openapi.ServerReadyJSONResponse{Message: "OK"},
	}, nil
}

func (server *OAPIStrictServerImpl) compute(
	ctx context.Context, localTrustRef *openapi.TrustMatrixRef,
	initialTrust *openapi.TrustVectorRef, preTrust *openapi.TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
	maxIterations *int, minIterations *int, checkFreq *int,
) (tv openapi.TrustVectorRef, flatTailStats openapi.FlatTailStats, err error) {
	logger := util.LoggerWithCaller(*zerolog.Ctx(ctx))
	var (
		c  *sparse.Matrix
		p  *sparse.Vector
		t0 *sparse.Vector
	)
	opts := []basic.ComputeOpt{basic.WithFlatTailStats(&flatTailStats)}
	if c, err = server.loadTrustMatrix(ctx, localTrustRef); err != nil {
		err = HTTPError{400, fmt.Errorf("cannot load local trust: %w", err)}
		return
	}
	cDim, err := c.Dim()
	if err != nil {
		return
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	if preTrust == nil {
		// Default to zero pre-trust (canonicalized into uniform later).
		p = sparse.NewVector(cDim, nil)
	} else if p, err = server.loadTrustVector(ctx, preTrust); err != nil {
		err = HTTPError{400, fmt.Errorf("cannot load pre-trust: %w", err)}
		return
	} else {
		// align dimensions
		switch {
		case p.Dim < cDim:
			p.SetDim(cDim)
		case cDim < p.Dim:
			cDim = p.Dim
			c.SetDim(p.Dim, p.Dim)
		}
	}
	logger.Trace().
		Int("dim", p.Dim).
		Int("nnz", p.NNZ()).
		Msg("pre-trust loaded")
	if initialTrust == nil {
		t0 = nil
	} else if t0, err = server.loadTrustVector(ctx, initialTrust); err != nil {
		err = HTTPError{400, fmt.Errorf("cannot load initial trust: %w", err)}
		return
	} else {
		// align dimensions
		switch {
		case t0.Dim < cDim:
			t0.SetDim(cDim)
		case cDim < t0.Dim:
			cDim = t0.Dim
			c.SetDim(t0.Dim, t0.Dim)
			p.SetDim(t0.Dim)
		}
		logger.Trace().
			Int("dim", t0.Dim).
			Int("nnz", t0.NNZ()).
			Msg("initial trust loaded")
		opts = append(opts, basic.WithInitialTrust(t0))
	}
	if alpha == nil {
		a := 0.5
		alpha = &a
	} else if *alpha < 0 || *alpha > 1 {
		err = HTTPError{
			400, fmt.Errorf("alpha=%f out of range [0..1]", *alpha),
		}
		return
	}
	if epsilon == nil {
		e := 1e-6 / float64(cDim)
		epsilon = &e
	} else if *epsilon <= 0 || *epsilon > 1 {
		err = HTTPError{
			400, fmt.Errorf("epsilon=%f out of range (0..1]", *epsilon),
		}
		return
	}
	if flatTail != nil {
		opts = append(opts, basic.WithFlatTail(*flatTail))
	}
	if numLeaders != nil {
		opts = append(opts, basic.WithFlatTailNumLeaders(*numLeaders))
	}
	if maxIterations != nil {
		opts = append(opts, basic.WithMaxIterations(*maxIterations))
	}
	if minIterations != nil {
		opts = append(opts, basic.WithMinIterations(*minIterations))
	}
	if checkFreq != nil {
		opts = append(opts, basic.WithCheckFreq(*checkFreq))
	}
	basic.CanonicalizeTrustVector(p)
	if t0 != nil {
		basic.CanonicalizeTrustVector(t0)
	}
	discounts, err := basic.ExtractDistrust(c)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot extract discounts: %w", err),
		}
		return
	}
	err = basic.CanonicalizeLocalTrust(c, p)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot canonicalize local trust: %w", err),
		}
		return
	}
	err = basic.CanonicalizeLocalTrust(discounts, nil)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot canonicalize discounts: %w", err),
		}
		return
	}
	t, err := basic.Compute(ctx, c, p, *alpha, *epsilon, opts...)
	c = nil
	p = nil
	runtime.GC()
	if err != nil {
		err = fmt.Errorf("cannot compute EigenTrust: %w", err)
		return
	}
	if err = basic.DiscountTrustVector(t, discounts); err != nil {
		err = fmt.Errorf("cannot apply local trust discounts: %w", err)
		return
	}
	itv := openapi.InlineTrustVector{
		Scheme: openapi.InlineTrustVectorSchemeInline,
		Size:   t.Dim,
	}
	for _, e := range t.Entries {
		itv.Entries = append(itv.Entries,
			openapi.InlineTrustVectorEntry{I: e.Index, V: e.Value})
	}
	if err = tv.FromInlineTrustVector(itv); err != nil {
		err = fmt.Errorf("cannot create response: %w", err)
		return
	}
	return tv, flatTailStats, nil
}

func (server *OAPIStrictServerImpl) Compute(
	ctx context.Context, request openapi.ComputeRequestObject,
) (openapi.ComputeResponseObject, error) {
	req := request.Body

	tv, _, err := server.compute(ctx,
		&req.LocalTrust, req.InitialTrust, req.PreTrust, req.Alpha, req.Epsilon,
		req.FlatTail, req.NumLeaders,
		req.MaxIterations, req.MinIterations, req.CheckFreq)
	if err != nil {
		var httpError HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 400:
				var resp openapi.Compute400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	resp := openapi.ComputeResponseOKJSONResponse(tv)
	return openapi.Compute200JSONResponse{resp}, nil
}

func (server *OAPIStrictServerImpl) ComputeWithStats(
	ctx context.Context, request openapi.ComputeWithStatsRequestObject,
) (openapi.ComputeWithStatsResponseObject, error) {
	req := request.Body
	tv, flatTailStats, err := server.compute(ctx,
		&req.LocalTrust, req.InitialTrust, req.PreTrust, req.Alpha, req.Epsilon,
		req.FlatTail, req.NumLeaders,
		req.MaxIterations, req.MinIterations, req.CheckFreq)
	if err != nil {
		var httpError HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 400:
				var resp openapi.ComputeWithStats400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	var resp openapi.ComputeWithStats200JSONResponse
	resp.EigenTrust = tv
	resp.FlatTailStats.Length = flatTailStats.Length
	resp.FlatTailStats.Threshold = flatTailStats.Threshold
	resp.FlatTailStats.DeltaNorm = flatTailStats.DeltaNorm
	resp.FlatTailStats.Ranking = flatTailStats.Ranking
	return resp, nil
}

func (server *OAPIStrictServerImpl) GetLocalTrust(
	ctx context.Context, request openapi.GetLocalTrustRequestObject,
) (openapi.GetLocalTrustResponseObject, error) {
	inline, err := server.getLocalTrust(ctx, request.Id)
	if err != nil {
		var httpError HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 404:
				return openapi.GetLocalTrust404Response{}, nil
			}
		}
		return nil, err
	}
	resp := openapi.LocalTrustGetResponseOKJSONResponse(*inline)
	return openapi.GetLocalTrust200JSONResponse{resp}, nil
}

func (server *OAPIStrictServerImpl) getLocalTrust(
	_ context.Context, id openapi.TrustCollectionId,
) (*openapi.InlineTrustMatrix, error) {
	tm, ok := server.core.storedTrustMatrix.Load(id)
	if !ok {
		return nil, HTTPError{Code: 404}
	}
	var (
		result openapi.InlineTrustMatrix
		err    error
	)
	tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) {
		result.Size, err = c.Dim()
		if err != nil {
			err = fmt.Errorf("cannot get dimension: %w", err)
			return
		}
		result.Entries = make([]openapi.InlineTrustMatrixEntry, 0, c.NNZ())
		for i, row := range c.Entries {
			for _, entry := range row {
				result.Entries = append(result.Entries,
					openapi.InlineTrustMatrixEntry{
						I: i,
						J: entry.Index,
						V: entry.Value,
					})
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (server *OAPIStrictServerImpl) HeadLocalTrust(
	ctx context.Context, request openapi.HeadLocalTrustRequestObject,
) (openapi.HeadLocalTrustResponseObject, error) {
	_, ok := server.core.storedTrustMatrix.Load(request.Id)
	if !ok {
		return openapi.HeadLocalTrust404Response{}, nil
	} else {
		return openapi.HeadLocalTrust204Response{}, nil
	}
}

func (server *OAPIStrictServerImpl) UpdateLocalTrust(
	ctx context.Context, request openapi.UpdateLocalTrustRequestObject,
) (openapi.UpdateLocalTrustResponseObject, error) {
	logger := util.LoggerWithCaller(*zerolog.Ctx(ctx))
	c, err := server.loadTrustMatrix(ctx, request.Body)
	if err != nil {
		return nil, HTTPError{
			400, fmt.Errorf("cannot load local trust: %w", err),
		}
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, fmt.Errorf("cannot get dimension: %w", err)
	}
	logger.Trace().
		Int("dim", cDim).
		Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	var (
		tm      *TrustMatrix
		created bool
	)
	if request.Params.Merge != nil && *request.Params.Merge {
		tm, created = server.core.storedTrustMatrix.Merge(request.Id, c)
	} else {
		tm, created = server.core.storedTrustMatrix.Set(request.Id, c)
	}
	tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) {
		err = c.Mmap(ctx)
		if err != nil {
			logger.Err(err).Msg("cannot swap out local trust")
		}
	})
	if created {
		return openapi.UpdateLocalTrust201Response{}, nil
	} else {
		return openapi.UpdateLocalTrust200Response{}, nil
	}
}

func (server *OAPIStrictServerImpl) DeleteLocalTrust(
	ctx context.Context, request openapi.DeleteLocalTrustRequestObject,
) (openapi.DeleteLocalTrustResponseObject, error) {
	_, deleted := server.core.storedTrustMatrix.LoadAndDelete(request.Id)
	if !deleted {
		return openapi.DeleteLocalTrust404Response{}, nil
	}
	return openapi.DeleteLocalTrust204Response{}, nil
}

func (server *OAPIStrictServerImpl) loadTrustMatrix(
	ctx context.Context,
	ref *openapi.TrustMatrixRef,
) (*sparse.Matrix, error) {
	switch ref.Scheme {
	case openapi.TrustMatrixRefSchemeInline:
		inline, err := ref.AsInlineTrustMatrix()
		if err != nil {
			return nil, err
		}
		return server.loadInlineTrustMatrix(&inline)
	case openapi.TrustMatrixRefSchemeStored:
		stored, err := ref.AsStoredTrustMatrix()
		if err != nil {
			return nil, err
		}
		return server.loadStoredTrustMatrix(&stored)
	case openapi.TrustMatrixRefSchemeObjectstorage:
		objectStorage, err := ref.AsObjectStorageTrustMatrix()
		if err != nil {
			return nil, err
		}
		return server.loadObjectStorageTrustMatrix(ctx, &objectStorage)
	default:
		return nil, fmt.Errorf("unknown local trust ref type %#v", ref.Scheme)
	}
}

func (server *OAPIStrictServerImpl) loadInlineTrustMatrix(
	inline *openapi.InlineTrustMatrix,
) (*sparse.Matrix, error) {
	if inline.Size <= 0 {
		return nil, fmt.Errorf("invalid size=%#v", inline.Size)
	}
	var entries []sparse.CooEntry
	for idx, entry := range inline.Entries {
		if entry.I < 0 || entry.I >= inline.Size {
			return nil, fmt.Errorf("entry %d: i=%d is out of range [0..%d)",
				idx, entry.I, inline.Size)
		}
		if entry.J < 0 || entry.J >= inline.Size {
			return nil, fmt.Errorf("entry %d: j=%d is out of range [0..%d)",
				idx, entry.J, inline.Size)
		}
		entries = append(entries, sparse.CooEntry{
			Row:    entry.I,
			Column: entry.J,
			Value:  entry.V,
		})
	}
	// reset after move
	size := inline.Size
	inline.Size = 0
	inline.Entries = nil
	return sparse.NewCSRMatrix(size, size, entries), nil
}

func (server *OAPIStrictServerImpl) loadStoredTrustMatrix(
	stored *openapi.StoredTrustMatrix,
) (c *sparse.Matrix, err error) {
	tm0, ok := server.core.storedTrustMatrix.Load(stored.Id)
	if ok {
		// Caller may modify returned c in-place (canonicalize, size-match)
		// so return a disposable copy, preserving the original.
		// This is slow: It takes ~3s to copy 16M nonzero entries.
		// TODO(ek): Implement on-demand canonicalization and remove this.
		tm0.LockAndRun(func(c0 *sparse.Matrix, timestamp *big.Int) {
			c = deepcopy.Copy(c0).(*sparse.Matrix)
		})
	} else {
		err = HTTPError{400, errors.New("trust matrix not found")}
	}
	return
}

func (server *OAPIStrictServerImpl) loadObjectStorageTrustMatrix(
	ctx context.Context, ref *openapi.ObjectStorageTrustMatrix,
) (*sparse.Matrix, error) {
	u, err := url.Parse(ref.Url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse object storage URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "s3":
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")
		return server.loadS3TrustMatrix(ctx, bucket, path)
	case "file":
		if !server.UseFileURI {
			return nil, fmt.Errorf("file: URI is disabled in this server")
		}
		return server.loadFileTrustMatrix(u.Path)
	default:
		return nil, fmt.Errorf("unknown object storage URL scheme %#v",
			u.Scheme)
	}
}

func (server *OAPIStrictServerImpl) loadS3TrustMatrix(
	ctx context.Context, bucket string, key string,
) (*sparse.Matrix, error) {
	res, err := server.core.loadS3Object(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("cannot load trust matrix from S3: %w", err)
	}
	defer util.Close(res.Body)
	return server.loadCsvTrustMatrix(csv.NewReader(res.Body))
}

func (server *OAPIStrictServerImpl) loadFileTrustMatrix(
	path string,
) (*sparse.Matrix, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return server.loadCsvTrustMatrix(csv.NewReader(f))
}

func (server *OAPIStrictServerImpl) loadCsvTrustMatrix(
	r *csv.Reader,
) (*sparse.Matrix, error) {
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV header: %w", err)
	}
	if !reflect.DeepEqual(header, []string{"i", "j", "v"}) {
		return nil, fmt.Errorf("invalid CSV header %#v", header)
	}
	var entries []sparse.CooEntry
	var size = 0
	for record, err := r.Read(); err == nil; record, err = r.Read() {
		if len(record) != 3 {
			return nil, fmt.Errorf("invalid CSV record %#v", record)
		}
		i, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, fmt.Errorf("invalid i=%#v: %w", record[0], err)
		}
		j, err := strconv.Atoi(record[1])
		if err != nil {
			return nil, fmt.Errorf("invalid j=%#v: %w", record[1], err)
		}
		v, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid v=%#v: %w", record[2], err)
		}
		if size < i+1 {
			size = i + 1
		}
		if size < j+1 {
			size = j + 1
		}
		entries = append(entries, sparse.CooEntry{Row: i, Column: j, Value: v})
	}
	return sparse.NewCSRMatrix(size, size, entries), nil
}

func (server *OAPIStrictServerImpl) loadTrustVector(
	ctx context.Context,
	ref *openapi.TrustVectorRef,
) (*sparse.Vector, error) {
	switch ref.Scheme {
	case openapi.Inline:
		inline, err := ref.AsInlineTrustVector()
		if err != nil {
			return nil, err
		}
		return loadInlineTrustVector(&inline)
	case openapi.Objectstorage:
		objectStorage, err := ref.AsObjectStorageTrustVector()
		if err != nil {
			return nil, err
		}
		return server.loadObjectStorageTrustVector(ctx, &objectStorage)
	default:
		return nil, fmt.Errorf("unknown trust vector ref type %#v", ref.Scheme)
	}
}

func loadInlineTrustVector(inline *openapi.InlineTrustVector) (
	*sparse.Vector, error,
) {
	var entries []sparse.Entry
	for idx, entry := range inline.Entries {
		if entry.I < 0 || entry.I >= inline.Size {
			return nil, fmt.Errorf("entry %d: i=%d is out of range [0..%d)",
				idx, entry.I, inline.Size)
		}
		if entry.V <= 0 {
			return nil, fmt.Errorf("entry %d: v=%f is out of range (0, inf)",
				idx, entry.V)
		}
		entries = append(entries, sparse.Entry{Index: entry.I, Value: entry.V})
	}
	// reset after move
	size := inline.Size
	inline.Size = 0
	inline.Entries = nil
	return sparse.NewVector(size, entries), nil
}

func (server *OAPIStrictServerImpl) loadObjectStorageTrustVector(
	ctx context.Context, ref *openapi.ObjectStorageTrustVector,
) (*sparse.Vector, error) {
	u, err := url.Parse(ref.Url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse object storage URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "s3":
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")
		return server.loadS3TrustVector(ctx, bucket, path)
	case "file":
		if !server.UseFileURI {
			return nil, fmt.Errorf("file: URI is disabled in this server")
		}
		return server.loadFileTrustVector(u.Path)
	default:
		return nil, fmt.Errorf("unknown object storage URL scheme %#v",
			u.Scheme)
	}
}

func (server *OAPIStrictServerImpl) loadS3TrustVector(
	ctx context.Context, bucket string, key string,
) (*sparse.Vector, error) {
	res, err := server.core.loadS3Object(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("cannot load trust vector from S3: %w", err)
	}
	defer util.Close(res.Body)
	r := csv.NewReader(res.Body)
	return server.loadCsvTrustVector(r)

}

func (server *OAPIStrictServerImpl) loadFileTrustVector(
	path string,
) (*sparse.Vector, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return server.loadCsvTrustVector(csv.NewReader(f))
}

func (server *OAPIStrictServerImpl) loadCsvTrustVector(
	r *csv.Reader,
) (*sparse.Vector, error) {
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV header: %w", err)
	}
	if !reflect.DeepEqual(header, []string{"i", "v"}) {
		return nil, fmt.Errorf("invalid CSV header %#v", header)
	}
	var entries []sparse.Entry
	var size = 0
	for record, err := r.Read(); err == nil; record, err = r.Read() {
		if len(record) != 2 {
			return nil, fmt.Errorf("invalid CSV record %#v", record)
		}
		i, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, fmt.Errorf("invalid i=%#v: %w", record[0], err)
		}
		v, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid v=%#v: %w", record[1], err)
		}
		if size < i+1 {
			size = i + 1
		}
		entries = append(entries, sparse.Entry{Index: i, Value: v})
	}
	return sparse.NewVector(size, entries), nil
}

func (server *Core) loadS3Object(
	ctx context.Context, bucket string, key string,
) (*s3.GetObjectOutput, error) {
	client := s3.NewFromConfig(server.awsConfig)
	region, err := manager.GetBucketRegion(ctx, client, bucket)
	if err != nil {
		return nil, fmt.Errorf("GetBucketRegion failed: %w", err)
	}
	awsConfig := server.awsConfig.Copy()
	awsConfig.Region = region
	client = s3.NewFromConfig(awsConfig)
	req := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	res, err := client.GetObject(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("GetObject failed: %w", err)
	}
	return res, nil
}
