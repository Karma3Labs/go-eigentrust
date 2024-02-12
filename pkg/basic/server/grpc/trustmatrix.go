package grpcserver

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	trustmatrixpb "k3l.io/go-eigentrust/pkg/api/pb/trustmatrix"
	"k3l.io/go-eigentrust/pkg/basic/server"
	"k3l.io/go-eigentrust/pkg/sparse"
)

type TrustMatrixServer struct {
	trustmatrixpb.UnimplementedServiceServer
	m *server.NamedTrustMatrices
}

func NewTrustMatrixServer(m *server.NamedTrustMatrices) *TrustMatrixServer {
	return &TrustMatrixServer{m: m}
}

func (svr *TrustMatrixServer) Create(
	ctx context.Context, request *trustmatrixpb.CreateRequest,
) (response *trustmatrixpb.CreateResponse, err error) {
	var id string
	if request.Id == "" {
		id, err = svr.m.New(ctx)
	} else {
		id = request.Id
		err = svr.m.NewNamed(request.Id)
	}
	if err == nil {
		response = &trustmatrixpb.CreateResponse{Id: id}
	}
	return
}

func (svr *TrustMatrixServer) Get(
	request *trustmatrixpb.GetRequest,
	server trustmatrixpb.Service_GetServer,
) error {
	tm, ok := svr.m.Load(request.Id)
	if !ok {
		return status.Error(codes.NotFound, "matrix not found")
	}
	return tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) error {
		if err := server.Send(&trustmatrixpb.GetResponse{
			Part: &trustmatrixpb.GetResponse_Header{
				Header: &trustmatrixpb.Header{
					Id:              &request.Id,
					TimestampQwords: BigUint2Qwords(timestamp),
				},
			},
		}); err != nil {
			return err
		}
		for i, row := range c.Entries {
			truster := strconv.Itoa(i)
			for _, entry := range row {
				if entry.Value == 0 {
					continue
				}
				if err := server.Send(&trustmatrixpb.GetResponse{
					Part: &trustmatrixpb.GetResponse_Entry{
						Entry: &trustmatrixpb.Entry{
							Truster: truster,
							Trustee: strconv.Itoa(entry.Index),
							Value:   entry.Value,
						},
					},
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (svr *TrustMatrixServer) Update(
	ctx context.Context, request *trustmatrixpb.UpdateRequest,
) (response *trustmatrixpb.UpdateResponse, err error) {
	logger := zerolog.Ctx(ctx)
	logger.Info().Interface("request", request)
	tm, ok := svr.m.Load(request.Header.GetId())
	if !ok {
		return nil, status.Error(codes.NotFound, "matrix not found")
	}
	err = tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) error {
		updateTimestamp := Qwords2BigUint(request.Header.TimestampQwords)
		if updateTimestamp.Cmp(timestamp) < 0 {
			return status.Error(codes.InvalidArgument, "stale update rejected")
		}
		var rows, cols int
		entries := make([]sparse.CooEntry, 0, len(request.Entries))
		for _, entry := range request.Entries {
			var (
				i, j int
				err1 error
			)
			if i, err1 = strconv.Atoi(entry.Truster); err1 != nil {
				return fmt.Errorf("invalid truster %#v: %w",
					entry.Truster, err1)
			}
			if j, err1 = strconv.Atoi(entry.Trustee); err1 != nil {
				return fmt.Errorf("invalid trustee %#v: %w",
					entry.Trustee, err1)
			}
			entries = append(entries, sparse.CooEntry{
				Row:    i,
				Column: j,
				Value:  entry.Value,
			})
			if rows <= i {
				rows = i + 1
			}
			if cols <= j {
				cols = j + 1
			}
		}
		switch {
		case rows < cols:
			rows = cols
		case cols < rows:
			cols = rows
		}
		c2 := sparse.NewCSRMatrix(rows, cols, entries, true)
		c.Merge(&c2.CSMatrix)
		if e := c.Mmap(ctx); e != nil {
			zerolog.Ctx(ctx).Err(e).Msg("cannot mmap")
		}
		timestamp.Set(updateTimestamp)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &trustmatrixpb.UpdateResponse{}, nil
}

func (svr *TrustMatrixServer) Flush(
	_ /*ctx*/ context.Context, request *trustmatrixpb.FlushRequest,
) (*trustmatrixpb.FlushResponse, error) {
	tm, ok := svr.m.Load(request.Id)
	if !ok {
		return nil, status.Error(codes.NotFound, "matrix not found")
	}
	_ = tm.LockAndRun(func(c *sparse.Matrix, timestamp *big.Int) error {
		c.Reset()
		timestamp.SetUint64(0)
		return nil
	})
	return &trustmatrixpb.FlushResponse{}, nil
}

func (svr *TrustMatrixServer) Delete(
	_ /*ctx*/ context.Context, request *trustmatrixpb.DeleteRequest,
) (*trustmatrixpb.DeleteResponse, error) {
	_, deleted := svr.m.LoadAndDelete(request.Id)
	if !deleted {
		return nil, status.Error(codes.NotFound, "matrix not found")
	}
	return &trustmatrixpb.DeleteResponse{}, nil
}
