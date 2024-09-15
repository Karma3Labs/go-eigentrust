package server

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func (server *Core) LoadS3Object(
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
