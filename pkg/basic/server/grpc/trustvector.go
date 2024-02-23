package grpcserver

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	trustvectorpb "k3l.io/go-eigentrust/pkg/api/pb/trustvector"
	"k3l.io/go-eigentrust/pkg/basic/server"
	"k3l.io/go-eigentrust/pkg/sparse"
	"k3l.io/go-eigentrust/pkg/util"
)

type TrustVectorServer struct {
	trustvectorpb.UnimplementedServiceServer
	v *server.NamedTrustVectors
}

func NewTrustVectorServer(v *server.NamedTrustVectors) *TrustVectorServer {
	return &TrustVectorServer{v: v}
}

func (svr *TrustVectorServer) Create(
	ctx context.Context, request *trustvectorpb.CreateRequest,
) (response *trustvectorpb.CreateResponse, err error) {
	var id string
	if request.Id == "" {
		id, err = svr.v.New(ctx)
	} else {
		id = request.Id
		err = svr.v.NewNamed(request.Id)
	}
	if err == nil {
		response = &trustvectorpb.CreateResponse{Id: id}
	}
	return
}

func (svr *TrustVectorServer) Get(
	request *trustvectorpb.GetRequest, server trustvectorpb.Service_GetServer,
) error {
	tv, ok := svr.v.Load(request.Id)
	if !ok {
		return status.Error(codes.NotFound, "vector not found")
	}
	return tv.LockAndRun(func(v *sparse.Vector, timestamp *big.Int) error {
		if err := server.Send(&trustvectorpb.GetResponse{
			Part: &trustvectorpb.GetResponse_Header{
				Header: &trustvectorpb.Header{
					Id:              &request.Id,
					TimestampQwords: BigUint2Qwords(timestamp),
				},
			},
		}); err != nil {
			return err
		}
		for _, entry := range v.Entries {
			if entry.Value == 0 {
				continue
			}
			if err := server.Send(&trustvectorpb.GetResponse{
				Part: &trustvectorpb.GetResponse_Entry{
					Entry: &trustvectorpb.Entry{
						Trustee: strconv.Itoa(entry.Index),
						Value:   entry.Value,
					},
				},
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (svr *TrustVectorServer) Update(
	ctx context.Context, request *trustvectorpb.UpdateRequest,
) (response *trustvectorpb.UpdateResponse, err error) {
	logger := util.LoggerWithCaller(*zerolog.Ctx(ctx))
	tv, ok := svr.v.Load(request.Header.GetId())
	if !ok {
		return nil, status.Error(codes.NotFound, "vector not found")
	}
	err = tv.LockAndRun(func(v *sparse.Vector, timestamp *big.Int) error {
		var (
			i, size int
			err1    error
		)
		entries := make([]sparse.Entry, 0, len(request.Entries))
		for _, entry := range request.Entries {
			if i, err1 = strconv.Atoi(entry.Trustee); err1 != nil {
				return fmt.Errorf("invalid truster %#v: %w",
					entry.Trustee, err1)
			}
			entries = append(entries, sparse.Entry{
				Index: i,
				Value: entry.Value,
			})
			if size <= i {
				size = i + 1
			}
		}
		v.Merge(sparse.NewVector(size, entries))
		updateTimestamp := Qwords2BigUint(request.Header.TimestampQwords)
		switch cmp := updateTimestamp.Cmp(timestamp); {
		case cmp > 0:
			timestamp.Set(updateTimestamp)
		case cmp < 0:
			logger.Warn().
				Str("updateTimestamp", updateTimestamp.String()).
				Str("vectorTimestamp", timestamp.String()).
				Msg("accepted stale update")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &trustvectorpb.UpdateResponse{}, nil
}

func (svr *TrustVectorServer) Flush(
	_ /*ctx*/ context.Context, request *trustvectorpb.FlushRequest,
) (*trustvectorpb.FlushResponse, error) {
	tv, ok := svr.v.Load(request.Id)
	if !ok {
		return nil, status.Error(codes.NotFound, "vector not found")
	}
	_ = tv.LockAndRun(func(v *sparse.Vector, timestamp *big.Int) error {
		v.Reset()
		timestamp.SetUint64(0)
		return nil
	})
	return &trustvectorpb.FlushResponse{}, nil
}

func (svr *TrustVectorServer) Delete(
	_ /*ctx*/ context.Context, request *trustvectorpb.DeleteRequest,
) (*trustvectorpb.DeleteResponse, error) {
	_, deleted := svr.v.LoadAndDelete(request.Id)
	if !deleted {
		return nil, status.Error(codes.NotFound, "vector not found")
	}
	return &trustvectorpb.DeleteResponse{}, nil
}
