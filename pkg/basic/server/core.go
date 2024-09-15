package server

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type Core struct {
	StoredTrustMatrices NamedTrustMatrices
	StoredTrustVectors  NamedTrustVectors
	awsConfig           aws.Config
}

func NewCore(ctx context.Context) (*Core, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot load AWS config: %w", err)
	}
	return &Core{
		awsConfig: awsConfig,
	}, nil
}
