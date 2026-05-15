package aws_s3

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type PresignerConfig struct {
	Region         string
	AccessKey      string
	SecretKey      string
	EndpointURL    string
	Bucket         string
	ObjectKey      string
	PostKey        string
	ExpiresSeconds int64
}

func NewPresignerFromEnv() (Presigner, PresignerConfig, error) {
	cfg := PresignerConfig{
		Region:         os.Getenv("AWS_REGION"),
		AccessKey:      os.Getenv("AWS_KEY"),
		SecretKey:      os.Getenv("AWS_SECRET"),
		EndpointURL:    os.Getenv("AWS_URL"),
		Bucket:         os.Getenv("AWS_BUCKET"),
		ObjectKey:      os.Getenv("AWS_OBJECT_KEY"),
		PostKey:        os.Getenv("AWS_POST_KEY"),
		ExpiresSeconds: 3600,
	}
	if cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.EndpointURL == "" || cfg.Bucket == "" || cfg.ObjectKey == "" {
		return Presigner{}, PresignerConfig{}, fmt.Errorf("missing AWS env configuration")
	}
	if exp := os.Getenv("AWS_PRESIGN_EXPIRES"); exp != "" {
		val, err := strconv.ParseInt(exp, 10, 64)
		if err != nil {
			return Presigner{}, PresignerConfig{}, fmt.Errorf("invalid AWS_PRESIGN_EXPIRES: %w", err)
		}
		cfg.ExpiresSeconds = val
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               cfg.EndpointURL,
					HostnameImmutable: true,
				}, nil
			}),
		),
	)
	if err != nil {
		return Presigner{}, PresignerConfig{}, err
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(s3Client)
	return Presigner{PresignClient: presignClient}, cfg, nil
}

