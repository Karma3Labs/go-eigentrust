package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	computepb "k3l.io/go-eigentrust/pkg/api/pb/compute"
	"k3l.io/go-eigentrust/pkg/basic/server"
)

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
