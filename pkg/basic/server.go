package basic

import "C"
import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mohae/deepcopy"
	"github.com/rs/zerolog"
	"k3l.io/go-eigentrust/pkg/sparse"
)

type StrictServerImpl struct {
	logger              zerolog.Logger
	storedLocalTrust    map[LocalTrustId]*sparse.Matrix
	storedLocalTrustMtx sync.Mutex
	awsConfig           aws.Config
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
		storedLocalTrust: make(map[LocalTrustId]*sparse.Matrix),
		awsConfig:        awsConfig,
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
	ctx context.Context, localTrustRef *LocalTrustRef,
	initialTrust *TrustVectorRef, preTrust *TrustVectorRef,
	alpha *float64, epsilon *float64,
	flatTail *int, numLeaders *int,
	maxIterations *int,
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
	if c, err = server.loadLocalTrust(ctx, localTrustRef); err != nil {
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
	DiscountTrustVector(t, discounts)
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
	tv, _, err := server.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		if httpError, ok := err.(HTTPError); ok {
			switch httpError.Code {
			case 400:
				var resp Compute400JSONResponse
				resp.Message = httpError.Inner.Error()
				return resp, nil
			}
		}
		return nil, err
	}
	return Compute200JSONResponse(tv), nil
}

func (server *StrictServerImpl) ComputeWithStats(
	ctx context.Context, request ComputeWithStatsRequestObject,
) (ComputeWithStatsResponseObject, error) {
	ctx = server.logger.WithContext(ctx)
	tv, flatTailStats, err := server.compute(ctx,
		&request.Body.LocalTrust,
		request.Body.InitialTrust, request.Body.PreTrust,
		request.Body.Alpha, request.Body.Epsilon,
		request.Body.FlatTail, request.Body.NumLeaders,
		request.Body.MaxIterations)
	if err != nil {
		if httpError, ok := err.(HTTPError); ok {
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
		if httpError, ok := err.(HTTPError); ok {
			switch httpError.Code {
			case 404:
				return GetLocalTrust404Response{}, nil
			}
		}
		return nil, err
	}
	return GetLocalTrust200JSONResponse(*inline), nil
}

func (server *StrictServerImpl) getLocalTrust(
	_ context.Context, id LocalTrustId,
) (*InlineLocalTrust, error) {
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
	entries := make([]InlineLocalTrustEntry, 0, c.NNZ())
	for i, row := range c.Entries {
		for _, entry := range row {
			entries = append(entries, InlineLocalTrustEntry{
				I: i,
				J: entry.Index,
				V: entry.Value,
			})
		}
	}
	return &InlineLocalTrust{
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
	c, err := server.loadLocalTrust(ctx, request.Body)
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

func (server *StrictServerImpl) loadLocalTrust(
	ctx context.Context,
	ref *LocalTrustRef,
) (*sparse.Matrix, error) {
	switch ref.Scheme {
	case LocalTrustRefSchemeInline:
		inline, err := ref.AsInlineLocalTrust()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse inline local trust reference: %w", err)
		}
		return server.loadInlineLocalTrust(&inline)
	case LocalTrustRefSchemeStored:
		stored, err := ref.AsStoredLocalTrust()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse stored local trust reference: %w", err)
		}
		return server.loadStoredLocalTrust(&stored)
	case LocalTrustRefSchemeObjectstorage:
		objectStorage, err := ref.AsObjectStorageLocalTrust()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse object storage local trust reference: %w", err)
		}
		return server.loadObjectStorageLocalTrust(ctx, &objectStorage)
	default:
		return nil, fmt.Errorf("unknown local trust ref type %#v", ref.Scheme)
	}
}

func (server *StrictServerImpl) loadInlineLocalTrust(
	inline *InlineLocalTrust,
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

func (server *StrictServerImpl) loadStoredLocalTrust(
	stored *StoredLocalTrust,
) (*sparse.Matrix, error) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	c, ok := server.storedLocalTrust[stored.Id]
	if !ok {
		return nil, HTTPError{400, errors.New("local trust not found")}
	}
	// Returned c is modified in-place (canonicalized, size-matched)
	// so return a disposable copy, preserving the original.
	// This is slow: It takes ~3s to copy 16M nonzero entries.
	// TODO(ek): Implement on-demand canonicalization and remove this.
	c = deepcopy.Copy(c).(*sparse.Matrix)
	return c, nil
}

func (server *StrictServerImpl) loadObjectStorageLocalTrust(
	ctx context.Context, ref *ObjectStorageLocalTrust,
) (*sparse.Matrix, error) {
	u, err := url.Parse(ref.Url)
	if err != nil {
		return nil, fmt.Errorf("cannot parse object storage URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "s3":
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")
		return server.loadS3LocalTrust(ctx, bucket, path)
	default:
		return nil, fmt.Errorf("unknown object storage URL scheme %#v",
			u.Scheme)
	}
}

func (server *StrictServerImpl) loadS3LocalTrust(
	ctx context.Context, bucket string, key string,
) (*sparse.Matrix, error) {
	client := s3.NewFromConfig(server.awsConfig)
	req := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	res, err := client.GetObject(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch from S3: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	return server.loadCsvLocalTrust(csv.NewReader(res.Body))
}

func (server *StrictServerImpl) loadCsvLocalTrust(
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
	case TrustVectorRefSchemeInline:
		inline, err := ref.AsInlineTrustVector()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot parse inline trust vector reference: %w", err)
		}
		return loadInlineTrustVector(&inline)
	case TrustVectorRefSchemeObjectstorage:
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
	default:
		return nil, fmt.Errorf("unknown object storage URL scheme %#v",
			u.Scheme)
	}
}

func (server *StrictServerImpl) loadS3TrustVector(
	ctx context.Context, bucket string, key string,
) (*sparse.Vector, error) {
	client := s3.NewFromConfig(server.awsConfig)
	req := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	res, err := client.GetObject(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch trust vector: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	r := csv.NewReader(res.Body)
	return server.loadCsvTrustVector(r)

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

func (server *StrictServerImpl) setStoredLocalTrust(
	id LocalTrustId, c *sparse.Matrix,
) (created bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	_, ok := server.storedLocalTrust[id]
	server.storedLocalTrust[id] = c
	return !ok
}

func (server *StrictServerImpl) mergeStoredLocalTrust(
	id LocalTrustId, c *sparse.Matrix,
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
	id LocalTrustId,
) (deleted bool) {
	server.storedLocalTrustMtx.Lock()
	defer server.storedLocalTrustMtx.Unlock()
	if _, ok := server.storedLocalTrust[id]; ok {
		delete(server.storedLocalTrust, id)
		deleted = true
	}
	return
}
