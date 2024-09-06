package basic

import "C"
import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mohae/deepcopy"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type StrictServerImpl struct {
	logger              zerolog.Logger
	storedLocalTrust    map[TrustCollectionId]*sparse.Matrix
	storedLocalTrustMtx sync.Mutex
	awsConfig           aws.Config
	UseFileURI          bool
}

func NewStrictServerImpl(
	ctx context.Context, logger zerolog.Logger,
) (*StrictServerImpl, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot load AWS config: %w", err)
	}
	return &StrictServerImpl{
		logger:           logger,
		storedLocalTrust: make(map[TrustCollectionId]*sparse.Matrix),
		awsConfig:        awsConfig,
		UseFileURI:       false,
	}, nil
}

func (server *StrictServerImpl) GetStatus(
	context.Context, GetStatusRequestObject,
) (GetStatusResponseObject, error) {
	return GetStatus200JSONResponse{ServerReadyJSONResponse{Message: "OK"}}, nil
}

type HTTPError struct {
	Code  int
	Inner error
}

func (e HTTPError) Error() string {
	statusText := http.StatusText(e.Code)
	if statusText != "" {
		statusText = " " + statusText
	}
	return fmt.Sprintf("HTTP %d%s: %s", e.Code, statusText, e.Inner.Error())
}

func (server *StrictServerImpl) compute(
	ctx context.Context, localTrustRef *TrustMatrixRef,
	initialTrust *TrustVectorRef, preTrust *TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
	maxIterations *int, minIterations *int, checkFreq *int,
) (tv TrustVectorRef, flatTailStats FlatTailStats, err error) {
	logger := server.logger.With().
		Str("func", "(*StrictServerImpl).Compute").
		Logger()
	var (
		c  *sparse.Matrix
		p  *sparse.Vector
		t0 *sparse.Vector
	)
	opts := []ComputeOpt{WithFlatTailStats(&flatTailStats)}
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
		opts = append(opts, WithInitialTrust(t0))
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
		opts = append(opts, WithFlatTail(*flatTail))
	}
	if numLeaders != nil {
		opts = append(opts, WithFlatTailNumLeaders(*numLeaders))
	}
	if maxIterations != nil {
		opts = append(opts, WithMaxIterations(*maxIterations))
	}
	if minIterations != nil {
		opts = append(opts, WithMinIterations(*minIterations))
	}
	if checkFreq != nil {
		opts = append(opts, WithCheckFreq(*checkFreq))
	}
	CanonicalizeTrustVector(p)
	if t0 != nil {
		CanonicalizeTrustVector(t0)
	}
	discounts, err := ExtractDistrust(c)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot extract discounts: %w", err),
		}
		return
	}
	err = CanonicalizeLocalTrust(c, p)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot canonicalize local trust: %w", err),
		}
		return
	}
	err = CanonicalizeLocalTrust(discounts, nil)
	if err != nil {
		err = HTTPError{
			400, fmt.Errorf("cannot canonicalize discounts: %w", err),
		}
		return
	}
	t, err := Compute(ctx, c, p, *alpha, *epsilon, opts...)
	c = nil
	p = nil
	runtime.GC()
	if err != nil {
		err = fmt.Errorf("cannot compute EigenTrust: %w", err)
		return
	}
	if err = DiscountTrustVector(t, discounts); err != nil {
		err = fmt.Errorf("cannot apply local trust discounts: %w", err)
		return
	}
	itv := InlineTrustVector{
		Scheme: InlineTrustVectorSchemeInline,
		Size:   t.Dim,
	}
	for _, e := range t.Entries {
		itv.Entries = append(itv.Entries,
			InlineTrustVectorEntry{I: e.Index, V: e.Value})
	}
	if err = tv.FromInlineTrustVector(itv); err != nil {
		err = fmt.Errorf("cannot create response: %w", err)
		return
	}
	return tv, flatTailStats, nil
}

func (server *StrictServerImpl) Compute(
	ctx context.Context, request ComputeRequestObject,
) (ComputeResponseObject, error) {
	ctx = server.logger.WithContext(ctx)
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
				var resp Compute400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	resp := ComputeResponseOKJSONResponse(tv)
	return Compute200JSONResponse{resp}, nil
}

func (server *StrictServerImpl) ComputeWithStats(
	ctx context.Context, request ComputeWithStatsRequestObject,
) (ComputeWithStatsResponseObject, error) {
	ctx = server.logger.WithContext(ctx)
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
				var resp ComputeWithStats400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	var resp ComputeWithStats200JSONResponse
	resp.EigenTrust = tv
	resp.FlatTailStats.Length = flatTailStats.Length
	resp.FlatTailStats.Threshold = flatTailStats.Threshold
	resp.FlatTailStats.DeltaNorm = flatTailStats.DeltaNorm
	resp.FlatTailStats.Ranking = flatTailStats.Ranking
	return resp, nil
}

func (server *StrictServerImpl) GetLocalTrust(
	ctx context.Context, request GetLocalTrustRequestObject,
) (GetLocalTrustResponseObject, error) {
	inline, err := server.getLocalTrust(ctx, request.Id)
	if err != nil {
		var httpError HTTPError
		if errors.As(err, &httpError) {
			switch httpError.Code {
			case 404:
				return GetLocalTrust404Response{}, nil
			}
		}
		return nil, err
	}
	resp := LocalTrustGetResponseOKJSONResponse(*inline)
	return GetLocalTrust200JSONResponse{resp}, nil
}

func (server *StrictServerImpl) getLocalTrust(
	_ context.Context, id TrustCollectionId,
) (*InlineTrustMatrix, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c := server.storedLocalTrust[id]
	if c == nil {
		return nil, HTTPError{Code: 404}
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, fmt.Errorf("cannot get dimension: %w", err)
	}
	entries := make([]InlineTrustMatrixEntry, 0, c.NNZ())
	for i, row := range c.Entries {
		for _, entry := range row {
			entries = append(entries, InlineTrustMatrixEntry{
				I: i,
				J: entry.Index,
				V: entry.Value,
			})
		}
	}
	return &InlineTrustMatrix{
		Entries: entries,
		Size:    cDim,
	}, nil
}

func (server *StrictServerImpl) HeadLocalTrust(
	_ context.Context, request HeadLocalTrustRequestObject,
) (HeadLocalTrustResponseObject, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	if server.storedLocalTrust[request.Id] == nil {
		return HeadLocalTrust404Response{}, nil
	} else {
		return HeadLocalTrust204Response{}, nil
	}
}

func (server *StrictServerImpl) UpdateLocalTrust(
	ctx context.Context, request UpdateLocalTrustRequestObject,
) (UpdateLocalTrustResponseObject, error) {
	logger := server.logger.With().
		Str("func", "(*StrictServerImpl).UpdateLocalTrust").
		Str("localTrustId", request.Id).
		Logger()
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
	var created bool
	if request.Params.Merge != nil && *request.Params.Merge {
		c, created = server.mergeStoredLocalTrust(request.Id, c)
	} else {
		created = server.setStoredLocalTrust(request.Id, c)
	}
	err = c.Mmap(ctx)
	if err != nil {
		logger.Err(err).Msg("cannot swap out local trust")
	}
	if created {
		return UpdateLocalTrust201Response{}, nil
	} else {
		return UpdateLocalTrust200Response{}, nil
	}
}

func (server *StrictServerImpl) DeleteLocalTrust(
	_ context.Context, request DeleteLocalTrustRequestObject,
) (DeleteLocalTrustResponseObject, error) {
	if !server.deleteStoredLocalTrust(request.Id) {
		return DeleteLocalTrust404Response{}, nil
	}
	return DeleteLocalTrust204Response{}, nil
}

func (server *StrictServerImpl) loadTrustMatrix(
	ctx context.Context,
	ref *TrustMatrixRef,
) (*sparse.Matrix, error) {
	switch ref.Scheme {
	case TrustMatrixRefSchemeInline:
		inline, err := ref.AsInlineTrustMatrix()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse inline local trust reference: %w", err)
		}
		return server.loadInlineTrustMatrix(&inline)
	case TrustMatrixRefSchemeStored:
		stored, err := ref.AsStoredTrustMatrix()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse stored local trust reference: %w", err)
		}
		return server.loadStoredTrustMatrix(&stored)
	case TrustMatrixRefSchemeObjectstorage:
		objectStorage, err := ref.AsObjectStorageTrustMatrix()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse object storage local trust reference: %w", err)
		}
		return server.loadObjectStorageTrustMatrix(ctx, &objectStorage)
	default:
		return nil, fmt.Errorf("unknown local trust ref type %#v", ref.Scheme)
	}
}

func (server *StrictServerImpl) loadInlineTrustMatrix(
	inline *InlineTrustMatrix,
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

func (server *StrictServerImpl) loadStoredTrustMatrix(
	stored *StoredTrustMatrix,
) (*sparse.Matrix, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c, ok := server.storedLocalTrust[stored.Id]
	if !ok {
		return nil, HTTPError{400, errors.New("trust matrix not found")}
	}
	// Returned c is modified in-place (canonicalized, size-matched)
	// so return a disposable copy, preserving the original.
	// This is slow: It takes ~3s to copy 16M nonzero entries.
	// TODO(ek): Implement on-demand canonicalization and remove this.
	c = deepcopy.Copy(c).(*sparse.Matrix)
	return c, nil
}

func (server *StrictServerImpl) loadObjectStorageTrustMatrix(
	ctx context.Context, ref *ObjectStorageTrustMatrix,
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

func (server *StrictServerImpl) loadS3TrustMatrix(
	ctx context.Context, bucket string, key string,
) (*sparse.Matrix, error) {
	res, err := server.loadS3Object(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("cannot load trust matrix from S3: %w", err)
	}
	defer util.Close(res.Body)
	return server.loadCsvTrustMatrix(csv.NewReader(res.Body))
}

func (server *StrictServerImpl) loadFileTrustMatrix(
	path string,
) (*sparse.Matrix, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return server.loadCsvTrustMatrix(csv.NewReader(f))
}

func (server *StrictServerImpl) loadCsvTrustMatrix(
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

func (server *StrictServerImpl) loadTrustVector(
	ctx context.Context,
	ref *TrustVectorRef,
) (*sparse.Vector, error) {
	switch ref.Scheme {
	case Inline:
		inline, err := ref.AsInlineTrustVector()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse inline trust vector reference: %w", err)
		}
		return loadInlineTrustVector(&inline)
	case Objectstorage:
		objectStorage, err := ref.AsObjectStorageTrustVector()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse object storage trust vector reference: %w", err)
		}
		return server.loadObjectStorageTrustVector(ctx, &objectStorage)
	default:
		return nil, fmt.Errorf("unknown trust vector ref type %#v", ref.Scheme)
	}
}

func loadInlineTrustVector(inline *InlineTrustVector) (*sparse.Vector, error) {
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

func (server *StrictServerImpl) loadObjectStorageTrustVector(
	ctx context.Context, ref *ObjectStorageTrustVector,
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

func (server *StrictServerImpl) loadS3TrustVector(
	ctx context.Context, bucket string, key string,
) (*sparse.Vector, error) {
	res, err := server.loadS3Object(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("cannot load trust vector from S3: %w", err)
	}
	defer util.Close(res.Body)
	r := csv.NewReader(res.Body)
	return server.loadCsvTrustVector(r)

}

func (server *StrictServerImpl) loadFileTrustVector(
	path string,
) (*sparse.Vector, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	return server.loadCsvTrustVector(csv.NewReader(f))
}

func (server *StrictServerImpl) loadCsvTrustVector(
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

func (server *StrictServerImpl) loadS3Object(
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

func (server *StrictServerImpl) setStoredLocalTrust(
	id TrustCollectionId, c *sparse.Matrix,
) (created bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	_, ok := server.storedLocalTrust[id]
	server.storedLocalTrust[id] = c
	return !ok
}

func (server *StrictServerImpl) mergeStoredLocalTrust(
	id TrustCollectionId, c *sparse.Matrix,
) (c2 *sparse.Matrix, created bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c2, ok := server.storedLocalTrust[id]
	if ok {
		c2.Merge(&c.CSMatrix)
	} else {
		server.storedLocalTrust[id] = c
	}
	return c2, !ok
}

func (server *StrictServerImpl) deleteStoredLocalTrust(
	id TrustCollectionId,
) (deleted bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	if _, ok := server.storedLocalTrust[id]; ok {
		delete(server.storedLocalTrust, id)
		deleted = true
	}
	return
}
