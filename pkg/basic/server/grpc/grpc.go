package grpc

import (
	"context"
	"math"
	"math/big"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	computepb "k3l.io/go-eigentrust/pkg/api/pb/compute"
	"k3l.io/go-eigentrust/pkg/basic/server"
)

var maxBigUint64 = new(big.Int).SetUint64(math.MaxUint64)

func BigUint2Qwords(value *big.Int) (qwords []uint64) {
	count := (value.BitLen() + 63) / 64
	qwords = make([]uint64, count)
	v := new(big.Int).Set(value)
	for count > 0 {
		count--
		qwords[count] = new(big.Int).And(v, maxBigUint64).Uint64()
		v.Rsh(v, 64)
	}
	return qwords
}

func Qwords2BigUint(qwords []uint64) (v *big.Int) {
	v = new(big.Int)
	for _, w := range qwords {
		v.Lsh(v, 64)
		v.Or(v, new(big.Int).SetUint64(w))
	}
	return v
}

type ComputeServer struct {
	computepb.UnimplementedServiceServer
	core *server.Core
}

func (svr *ComputeServer) BasicCompute(
	_ /*ctx*/ context.Context, _ /*request*/ *computepb.BasicComputeRequest,
) (*computepb.BasicComputeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method not implemented")
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
