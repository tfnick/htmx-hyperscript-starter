package oss

import (
	"context"
	"io"
	"time"
)

const (
	Scenario               = "oss"
	OperationPutObject     = "put_object"
	OperationGetObject     = "get_object"
	OperationDeleteObject  = "delete_object"
	OperationPresignObject = "presign_object"
)

type ProviderConfig struct {
	ChannelCode     string
	ProviderCode    string
	AdapterKey      string
	EndpointURL     string
	Bucket          string
	Region          string
	PublicBaseURL   string
	KeyPrefix       string
	UsePathStyle    *bool
	AccessKeyID     string
	SecretAccessKey string
}

type PutObjectRequest struct {
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
	Metadata    map[string]string
}

type PutObjectResult struct {
	Key               string
	ETag              string
	Size              int64
	PublicURL         string
	ProviderRequestID string
}

type GetObjectRequest struct {
	Key string
}

type GetObjectResult struct {
	Key               string
	Body              io.ReadCloser
	ContentType       string
	Size              int64
	ETag              string
	Metadata          map[string]string
	ProviderRequestID string
}

type DeleteObjectRequest struct {
	Key string
}

type DeleteObjectResult struct {
	Deleted           bool
	ProviderRequestID string
}

type PresignObjectRequest struct {
	Key         string
	Method      string
	ExpiresIn   time.Duration
	ContentType string
}

type PresignObjectResult struct {
	URL               string
	ExpiresAt         time.Time
	ProviderRequestID string
}

type Adapter interface {
	PutObject(ctx context.Context, cfg ProviderConfig, req PutObjectRequest) (PutObjectResult, error)
	GetObject(ctx context.Context, cfg ProviderConfig, req GetObjectRequest) (GetObjectResult, error)
	DeleteObject(ctx context.Context, cfg ProviderConfig, req DeleteObjectRequest) (DeleteObjectResult, error)
	PresignObject(ctx context.Context, cfg ProviderConfig, req PresignObjectRequest) (PresignObjectResult, error)
}
