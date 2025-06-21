# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go implementation of the EigenTrust algorithm with both server and client implementations. EigenTrust is a reputation-based trust management system that computes global trust values from local trust opinions.

## Development Commands

### Build and Install
```bash
# Install the main binary
go install ./cmd/eigentrust

# Install all development tools
./scripts/install_build_tools.sh
```

### Code Generation
```bash
# Generate all code (protobuf and OpenAPI)
./scripts/gogenerate.sh

# Manual generation for specific components:
# - Protocol buffers: see pkg/api/pb/generate.go
# - OpenAPI client: see pkg/api/openapi/generate.go
```

### Testing and Linting
```bash
# Run tests
go test ./...

# Run linter (golangci-lint is installed via tools.go)
golangci-lint run

# Check protocol buffer linting
protolint api/pb/*.proto
```

### Running the Application
```bash
# Start the server
eigentrust serve

# Run basic EigenTrust computation
eigentrust basic compute -L -l examples/simple-lt.csv -p examples/simple-pt.csv

# Run with custom alpha (pre-trust weight)
eigentrust basic compute -L -l lt.csv -p pt.csv -a 0.01
```

## Architecture Overview

### Core Packages

- **pkg/basic**: Core EigenTrust algorithm implementation
  - `eigentrust.go`: Main algorithm logic and convergence checking
  - `localtrust.go`: Local trust matrix handling  
  - `trustvector.go`: Trust vector operations
  - `server/`: HTTP and gRPC server implementations

- **pkg/sparse**: Sparse matrix/vector operations optimized for EigenTrust
  - `matrix.go`: Compressed sparse matrix (CSR/CSC format)
  - `vector.go`: Sparse vector operations
  - `option/`: Configuration options for sparse operations

- **pkg/peer**: Peer identity management and parsing
- **pkg/util**: Common utilities (CSV parsing, logging, file operations)
- **cmd/eigentrust**: CLI application with subcommands

### API Layer

- **api/pb/**: Protocol buffer definitions for gRPC services
- **api/openapi/**: OpenAPI/REST API specification
- **pkg/api/**: Generated Go code from protobuf and OpenAPI specs

### Key Algorithms

The EigenTrust implementation uses:
- Sparse matrix operations for efficiency with large, sparse trust networks
- Convergence checking with configurable epsilon thresholds
- Support for pre-trust (alpha parameter) to bootstrap trust computation
- Both synchronous and iterative computation modes

### Server Architecture

The server supports both HTTP/REST and gRPC APIs:
- REST endpoints generated from OpenAPI spec
- gRPC services for compute, trust matrix, and trust vector operations
- AWS S3 integration for storing large trust matrices/vectors
- Periodic recomputation jobs

## Input Format

The system expects CSV input files:
- **Local Trust**: `from,to,value` format showing peer-to-peer trust relationships
- **Pre-Trust**: `peer_id,value` format defining initial trust distribution

Example files are available in the `examples/` directory for testing different network topologies.

## Migration to ogen (In Progress)

The project is migrating from oapi-codegen to ogen for better OpenAPI 3.1 support:

### Current Status
- ✅ ogen code generation setup (`pkg/api/ogen/`)
- ✅ Type conversion utilities (`pkg/api/convert/`)
- ✅ ogen handler implementation (`pkg/basic/server/ogen/`)
- 🔄 Server integration with dual support
- ⏳ Testing and validation

### Using ogen Server (Experimental)
```bash
# Start server with ogen-generated handlers
eigentrust serve --use-ogen

# Compare with existing oapi-codegen implementation
eigentrust serve  # (default, stable)
```

### Migration Benefits
- **OpenAPI 3.1 Support**: Native support for latest OpenAPI spec
- **Better Type Safety**: Compile-time union type checking instead of runtime
- **Performance**: Zero-allocation JSON handling
- **Future-proof**: Active development and maintenance