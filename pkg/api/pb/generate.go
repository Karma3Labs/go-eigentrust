package pb

//go:generate protoc --proto_path=../../../api/pb --go_out=. --go_opt=module=k3l.io/go-eigentrust/pkg/api/pb --go-grpc_out=. --go-grpc_opt=module=k3l.io/go-eigentrust/pkg/api/pb ../../../api/pb/trustvector.proto ../../../api/pb/trustmatrix.proto ../../../api/pb/compute.proto
