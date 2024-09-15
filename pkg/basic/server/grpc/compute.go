package grpcserver

import (
	"context"
	"math/big"
	"runtime"

	"github.com/mohae/deepcopy"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	computepb "k3l.io/go-eigentrust/pkg/api/pb/compute"
	"k3l.io/go-eigentrust/pkg/basic"
	"k3l.io/go-eigentrust/pkg/basic/server"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type ComputeServer struct {
	computepb.UnimplementedServiceServer
	core *server.Core
}

func (svr *ComputeServer) BasicCompute(
	ctx context.Context, request *computepb.BasicComputeRequest,
) (*computepb.BasicComputeResponse, error) {
	// TODO(ek): Copied from OpenAPI-side code; refactor.
	logger := util.LoggerWithCaller(*zerolog.Ctx(ctx))
	var (
		c   *sparse.Matrix
		p   *sparse.Vector
		t   *sparse.Vector
		gt  *server.TrustVector
		ts  = &big.Int{}
		ok  bool
		err error
	)
	opts := []basic.ComputeOpt{}
	if lt, ok := svr.core.StoredTrustMatrices.Load(request.Params.LocalTrustId); ok {
		_ = lt.LockAndRun(func(c1 *sparse.Matrix, timestamp *big.Int) error {
			logger.Info().
				Str("id", request.Params.LocalTrustId).
				Interface("c1", c1).
				Interface("timestamp", timestamp).
				Msg("local trust")
			c = deepcopy.Copy(c1).(*sparse.Matrix)
			if ts.Cmp(timestamp) < 0 {
				ts.Set(timestamp)
			}
			return nil
		})
	} else {
		return nil, status.Error(codes.NotFound, "local trust not found")
	}
	cDim, err := c.Dim()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "c is not square: %#v*%#v",
			c.MajorDim, c.MinorDim)
	}
	if request.Params.PreTrustId == "" {
		p = sparse.NewVector(cDim, nil)
	} else if pt, ok := svr.core.StoredTrustVectors.Load(request.Params.PreTrustId); ok {
		_ = pt.LockAndRun(func(p1 *sparse.Vector, timestamp *big.Int) error {
			p = deepcopy.Copy(p1).(*sparse.Vector)
			if ts.Cmp(timestamp) < 0 {
				ts.Set(timestamp)
			}
			return nil
		})
		switch {
		case p.Dim < cDim:
			p.SetDim(cDim)
		case cDim < p.Dim:
			cDim = p.Dim
			c.SetDim(p.Dim, p.Dim)
		}
	} else {
		return nil, status.Error(codes.NotFound, "pre-trust not found")
	}
	if gt, ok = svr.core.StoredTrustVectors.Load(request.Params.GlobalTrustId); ok {
		_ = gt.LockAndRun(func(
			t1 *sparse.Vector, timestamp *big.Int,
		) error {
			t = deepcopy.Copy(t1).(*sparse.Vector)
			if ts.Cmp(timestamp) < 0 {
				ts.Set(timestamp)
			}
			return nil
		})
		switch {
		case t.Dim < p.Dim:
			t.SetDim(p.Dim)
		case p.Dim < t.Dim:
			p.SetDim(t.Dim)
			cDim = t.Dim
			c.SetDim(t.Dim, t.Dim)
		}
	} else {
		return nil, status.Error(codes.NotFound, "global trust not found")
	}
	opts = append(opts, basic.WithInitialTrust(t), basic.WithResultIn(t))
	logger.Info().Int("dim", cDim).Int("nnz", c.NNZ()).
		Msg("local trust loaded")
	logger.Info().Int("dim", p.Dim).Int("nnz", p.NNZ()).
		Msg("pre-trust loaded")
	logger.Info().Int("dim", t.Dim).Int("nnz", t.NNZ()).
		Msg("global/initial trust loaded")
	alpha := request.Params.Alpha
	if alpha == nil {
		a := 0.5
		alpha = &a
	} else if *alpha < 0 || *alpha > 1 {
		return nil, status.Errorf(codes.InvalidArgument,
			"alpha=%f out of range [0..1]", *alpha)
	}
	epsilon := request.Params.Epsilon
	if epsilon == nil {
		e := 1e-6 / float64(cDim)
		epsilon = &e
	} else if *epsilon <= 0 || *epsilon > 1 {
		return nil, status.Errorf(codes.InvalidArgument,
			"epsilon=%f out of range (0..1]", *epsilon)
	}
	basic.CanonicalizeTrustVector(p)
	basic.CanonicalizeTrustVector(t)
	discounts, err := basic.ExtractDistrust(c)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"cannot extract discounts: %s", err.Error())
	}
	err = basic.CanonicalizeLocalTrust(c, p)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"cannot canonicalize local trust: %s", err.Error())
	}
	err = basic.CanonicalizeLocalTrust(discounts, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"cannot canonicalize discounts: %s", err.Error())
	}
	_, err = basic.Compute(ctx, c, p, *alpha, *epsilon, opts...)
	c = nil
	p = nil
	runtime.GC()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable,
			"cannot compute EigenTrust: %s", err.Error())
	}
	if request.Params.PositiveGlobalTrustId != "" {
		if gtp, ok := svr.core.StoredTrustVectors.Load(request.Params.PositiveGlobalTrustId); ok {
			_ = gtp.LockAndRun(func(
				tp *sparse.Vector, timestamp *big.Int,
			) error {
				tp.Assign(t)
				if timestamp.Cmp(ts) < 0 {
					timestamp.Set(ts)
				}
				return nil
			})
		} else {
			logger.Warn().
				Str("id", request.Params.PositiveGlobalTrustId).
				Msg("positive global trust vector not found")
		}
	}
	basic.DiscountTrustVector(t, discounts)
	_ = gt.LockAndRun(func(t1 *sparse.Vector, timestamp *big.Int) error {
		t1.Assign(t)
		if timestamp.Cmp(ts) < 0 {
			timestamp.Set(ts)
		}
		return nil
	})
	return &computepb.BasicComputeResponse{}, nil
}

func (svr *ComputeServer) CreateJob(
	_ /*ctx*/ context.Context,
	_ /*request*/ *computepb.CreateJobRequest,
) (*computepb.CreateJobResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

func (svr *ComputeServer) DeleteJob(
	_ /*ctx*/ context.Context,
	_ /*request*/ *computepb.DeleteJobRequest,
) (*computepb.DeleteJobResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

func NewGrpcServer(core *server.Core) *ComputeServer {
	return &ComputeServer{core: core}
}
