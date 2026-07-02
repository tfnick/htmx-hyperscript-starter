package s3compatible

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithymiddleware "github.com/aws/smithy-go/middleware"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

const defaultRegion = "auto"

type S3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type PresignAPI interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type Option func(*Adapter)

type Adapter struct {
	httpClient     *http.Client
	clientFactory  func(context.Context, oss.ProviderConfig) (S3API, error)
	presignFactory func(context.Context, oss.ProviderConfig) (PresignAPI, error)
	now            func() time.Time
}

func NewAdapter(httpClient *http.Client) *Adapter {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Adapter{
		httpClient: httpClient,
		now:        time.Now,
	}
}

func NewAdapterWithOptions(httpClient *http.Client, options ...Option) *Adapter {
	adapter := NewAdapter(httpClient)
	for _, option := range options {
		option(adapter)
	}
	return adapter
}

func WithClientFactory(factory func(context.Context, oss.ProviderConfig) (S3API, error)) Option {
	return func(a *Adapter) {
		a.clientFactory = factory
	}
}

func WithPresignFactory(factory func(context.Context, oss.ProviderConfig) (PresignAPI, error)) Option {
	return func(a *Adapter) {
		a.presignFactory = factory
	}
}

func (a *Adapter) PutObject(ctx context.Context, cfg oss.ProviderConfig, req oss.PutObjectRequest) (oss.PutObjectResult, error) {
	if req.Body == nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryValidation, false, "OSS object body is required", nil)
	}
	if err := validateConfig(cfg); err != nil {
		return oss.PutObjectResult{}, err
	}
	key, providerKey, err := objectKeys(cfg, req.Key)
	if err != nil {
		return oss.PutObjectResult{}, err
	}
	client, err := a.s3Client(ctx, cfg)
	if err != nil {
		return oss.PutObjectResult{}, err
	}

	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	metadata := safeMetadata(req.Metadata)
	result, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(strings.TrimSpace(cfg.Bucket)),
		Key:         aws.String(providerKey),
		Body:        req.Body,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	})
	if err != nil {
		return oss.PutObjectResult{}, providerError(err)
	}

	size := req.Size
	if size < 0 {
		size = 0
	}
	return oss.PutObjectResult{
		Key:               key,
		ETag:              trimHeaderQuotes(aws.ToString(result.ETag)),
		Size:              size,
		PublicURL:         publicURL(cfg.PublicBaseURL, providerKey),
		ProviderRequestID: responseRequestID(result.ResultMetadata),
	}, nil
}

func (a *Adapter) GetObject(ctx context.Context, cfg oss.ProviderConfig, req oss.GetObjectRequest) (oss.GetObjectResult, error) {
	if err := validateConfig(cfg); err != nil {
		return oss.GetObjectResult{}, err
	}
	key, providerKey, err := objectKeys(cfg, req.Key)
	if err != nil {
		return oss.GetObjectResult{}, err
	}
	client, err := a.s3Client(ctx, cfg)
	if err != nil {
		return oss.GetObjectResult{}, err
	}

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(strings.TrimSpace(cfg.Bucket)),
		Key:    aws.String(providerKey),
	})
	if err != nil {
		return oss.GetObjectResult{}, providerError(err)
	}
	if result.Body == nil {
		return oss.GetObjectResult{}, providererror.New(providererror.CategoryProviderInternal, false, "OSS object body is empty", nil)
	}

	return oss.GetObjectResult{
		Key:               key,
		Body:              result.Body,
		ContentType:       aws.ToString(result.ContentType),
		Size:              aws.ToInt64(result.ContentLength),
		ETag:              trimHeaderQuotes(aws.ToString(result.ETag)),
		Metadata:          result.Metadata,
		ProviderRequestID: responseRequestID(result.ResultMetadata),
	}, nil
}

func (a *Adapter) DeleteObject(ctx context.Context, cfg oss.ProviderConfig, req oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	if err := validateConfig(cfg); err != nil {
		return oss.DeleteObjectResult{}, err
	}
	_, providerKey, err := objectKeys(cfg, req.Key)
	if err != nil {
		return oss.DeleteObjectResult{}, err
	}
	client, err := a.s3Client(ctx, cfg)
	if err != nil {
		return oss.DeleteObjectResult{}, err
	}

	result, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(strings.TrimSpace(cfg.Bucket)),
		Key:    aws.String(providerKey),
	})
	if err != nil {
		mappedErr := providerError(err)
		if providerErr, ok := providererror.From(mappedErr); ok && providerErr.Category == providererror.CategoryPermanent {
			return oss.DeleteObjectResult{Deleted: false, ProviderRequestID: providerErr.ProviderRequestID}, nil
		}
		return oss.DeleteObjectResult{}, mappedErr
	}
	return oss.DeleteObjectResult{Deleted: true, ProviderRequestID: responseRequestID(result.ResultMetadata)}, nil
}

func (a *Adapter) PresignObject(ctx context.Context, cfg oss.ProviderConfig, req oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	if err := validateConfig(cfg); err != nil {
		return oss.PresignObjectResult{}, err
	}
	_, providerKey, err := objectKeys(cfg, req.Key)
	if err != nil {
		return oss.PresignObjectResult{}, err
	}
	presigner, err := a.presigner(ctx, cfg)
	if err != nil {
		return oss.PresignObjectResult{}, err
	}
	expiresIn := req.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	var result *v4.PresignedHTTPRequest
	switch method {
	case http.MethodGet:
		result, err = presigner.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(strings.TrimSpace(cfg.Bucket)),
			Key:    aws.String(providerKey),
		}, s3.WithPresignExpires(expiresIn))
	case http.MethodPut:
		contentType := strings.TrimSpace(req.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		result, err = presigner.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(strings.TrimSpace(cfg.Bucket)),
			Key:         aws.String(providerKey),
			ContentType: aws.String(contentType),
		}, s3.WithPresignExpires(expiresIn))
	default:
		return oss.PresignObjectResult{}, providererror.New(providererror.CategoryValidation, false, "OSS presign method is not supported", nil)
	}
	if err != nil {
		return oss.PresignObjectResult{}, providerError(err)
	}

	return oss.PresignObjectResult{
		URL:               result.URL,
		ExpiresAt:         a.now().UTC().Add(expiresIn),
		ProviderRequestID: "",
	}, nil
}

func (a *Adapter) s3Client(ctx context.Context, cfg oss.ProviderConfig) (S3API, error) {
	if a.clientFactory != nil {
		return a.clientFactory(ctx, cfg)
	}
	return a.newS3Client(ctx, cfg)
}

func (a *Adapter) presigner(ctx context.Context, cfg oss.ProviderConfig) (PresignAPI, error) {
	if a.presignFactory != nil {
		return a.presignFactory(ctx, cfg)
	}
	client, err := a.newS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return s3.NewPresignClient(client), nil
}

func (a *Adapter) newS3Client(ctx context.Context, cfg oss.ProviderConfig) (*s3.Client, error) {
	endpoint, err := normalizedEndpoint(cfg.EndpointURL)
	if err != nil {
		return nil, err
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = defaultRegion
	}

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)),
		config.WithHTTPClient(a.httpClient),
	)
	if err != nil {
		return nil, providererror.New(providererror.CategoryProviderInternal, false, "failed to configure OSS provider", err)
	}

	return s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = usePathStyle(cfg)
	}), nil
}

func validateConfig(cfg oss.ProviderConfig) error {
	if _, err := normalizedEndpoint(cfg.EndpointURL); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return providererror.New(providererror.CategoryValidation, false, "OSS bucket is required", nil)
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return providererror.New(providererror.CategoryAuth, false, "OSS credential is required", nil)
	}
	return nil
}

func normalizedEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", providererror.New(providererror.CategoryValidation, false, "OSS endpoint URL is invalid", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", providererror.New(providererror.CategoryValidation, false, "OSS endpoint URL is invalid", nil)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func usePathStyle(cfg oss.ProviderConfig) bool {
	if cfg.UsePathStyle != nil {
		return *cfg.UsePathStyle
	}
	return strings.EqualFold(strings.TrimSpace(cfg.ProviderCode), "cloudflare_r2")
}

func objectKeys(cfg oss.ProviderConfig, rawKey string) (string, string, error) {
	key := cleanObjectKey(rawKey)
	if key == "" {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS object key is required", nil)
	}

	prefix := cleanObjectKey(cfg.KeyPrefix)
	if prefix == "" {
		return key, key, nil
	}
	return key, prefix + "/" + key, nil
}

func cleanObjectKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	key = filepath.ToSlash(filepath.Clean(key))
	if key == "." || key == ".." || strings.HasPrefix(key, "../") {
		return ""
	}
	return key
}

func safeMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cleanKey := strings.TrimSpace(key)
		if cleanKey == "" {
			continue
		}
		result[cleanKey] = strings.TrimSpace(value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func publicURL(baseURL string, key string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	segments := strings.Split(cleanObjectKey(key), "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return baseURL + "/" + strings.Join(escaped, "/")
}

func providerError(err error) error {
	if err == nil {
		return nil
	}

	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return withProviderDetails(providererror.New(providererror.CategoryPermanent, false, "OSS object not found", err), err)
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		category, retryable := categoryForAPIError(apiErr.ErrorCode())
		return withProviderDetails(providererror.New(category, retryable, safeMessageForAPIError(apiErr.ErrorCode()), err), err)
	}

	return providererror.New(providererror.CategoryTemporary, true, "OSS provider request failed", err)
}

func categoryForAPIError(code string) (string, bool) {
	switch code {
	case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "InvalidSecurity":
		return providererror.CategoryAuth, false
	case "NoSuchBucket", "NoSuchKey", "NotFound", "404":
		return providererror.CategoryPermanent, false
	case "SlowDown", "TooManyRequests", "Throttling", "ThrottlingException":
		return providererror.CategoryRateLimit, true
	case "RequestTimeout", "RequestTimeoutException":
		return providererror.CategoryTimeout, true
	case "InvalidArgument", "InvalidBucketName", "InvalidObjectName", "MalformedXML":
		return providererror.CategoryValidation, false
	case "InternalError", "ServiceUnavailable":
		return providererror.CategoryTemporary, true
	default:
		return providererror.CategoryProviderInternal, false
	}
}

func safeMessageForAPIError(code string) string {
	switch code {
	case "NoSuchKey", "NotFound", "404":
		return "OSS object not found"
	case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch", "InvalidSecurity":
		return "OSS provider authentication failed"
	case "SlowDown", "TooManyRequests", "Throttling", "ThrottlingException":
		return "OSS provider rate limit exceeded"
	default:
		return "OSS provider request failed"
	}
}

func withProviderDetails(providerErr *providererror.Error, err error) *providererror.Error {
	var responseErr interface {
		ServiceRequestID() string
	}
	if errors.As(err, &responseErr) {
		providerErr.ProviderRequestID = responseErr.ServiceRequestID()
	}
	return providerErr
}

func responseRequestID(metadata smithymiddleware.Metadata) string {
	requestID, _ := awsmiddleware.GetRequestIDMetadata(metadata)
	return requestID
}

func trimHeaderQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}
