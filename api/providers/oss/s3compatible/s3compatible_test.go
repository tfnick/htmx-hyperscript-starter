package s3compatible

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type fakeS3Client struct {
	putInput     *s3.PutObjectInput
	getInput     *s3.GetObjectInput
	deleteInput  *s3.DeleteObjectInput
	putOutput    *s3.PutObjectOutput
	getOutput    *s3.GetObjectOutput
	deleteOutput *s3.DeleteObjectOutput
	err          error
}

func (c *fakeS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.putInput = input
	if c.err != nil {
		return nil, c.err
	}
	if c.putOutput != nil {
		return c.putOutput, nil
	}
	return &s3.PutObjectOutput{ETag: aws.String(`"etag-put"`)}, nil
}

func (c *fakeS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.getInput = input
	if c.err != nil {
		return nil, c.err
	}
	if c.getOutput != nil {
		return c.getOutput, nil
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("image-bytes")),
		ContentLength: aws.Int64(int64(len("image-bytes"))),
		ContentType:   aws.String("image/png"),
		ETag:          aws.String(`"etag-get"`),
	}, nil
}

func (c *fakeS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	c.deleteInput = input
	if c.err != nil {
		return nil, c.err
	}
	if c.deleteOutput != nil {
		return c.deleteOutput, nil
	}
	return &s3.DeleteObjectOutput{}, nil
}

type fakePresignClient struct {
	getInput *s3.GetObjectInput
	putInput *s3.PutObjectInput
	expires  time.Duration
}

func (c *fakePresignClient) PresignGetObject(_ context.Context, input *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	c.getInput = input
	c.expires = presignExpires(optFns...)
	return &v4.PresignedHTTPRequest{URL: "https://signed.example.com/get", Method: http.MethodGet}, nil
}

func (c *fakePresignClient) PresignPutObject(_ context.Context, input *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	c.putInput = input
	c.expires = presignExpires(optFns...)
	return &v4.PresignedHTTPRequest{URL: "https://signed.example.com/put", Method: http.MethodPut}, nil
}

func TestPutObjectUsesAWSSDKClientInputs(t *testing.T) {
	client := &fakeS3Client{}
	adapter := NewAdapterWithOptions(nil, WithClientFactory(func(context.Context, oss.ProviderConfig) (S3API, error) {
		return client, nil
	}))

	result, err := adapter.PutObject(t.Context(), testProviderConfig(), oss.PutObjectRequest{
		Key:         "settings/site-logo.png",
		Body:        strings.NewReader("logo"),
		Size:        4,
		ContentType: "image/png",
		Metadata:    map[string]string{"filename": " logo.png "},
	})
	if err != nil {
		t.Fatalf("put object: %v", err)
	}

	if aws.ToString(client.putInput.Bucket) != "assets" {
		t.Fatalf("unexpected bucket: %q", aws.ToString(client.putInput.Bucket))
	}
	if aws.ToString(client.putInput.Key) != "public/settings/site-logo.png" {
		t.Fatalf("unexpected object key: %q", aws.ToString(client.putInput.Key))
	}
	if aws.ToString(client.putInput.ContentType) != "image/png" {
		t.Fatalf("unexpected content type: %q", aws.ToString(client.putInput.ContentType))
	}
	if client.putInput.Metadata["filename"] != "logo.png" {
		t.Fatalf("unexpected metadata: %#v", client.putInput.Metadata)
	}
	if result.Key != "settings/site-logo.png" || result.ETag != "etag-put" || result.Size != 4 {
		t.Fatalf("unexpected put result: %#v", result)
	}
	if result.PublicURL != "https://assets.example.com/public/settings/site-logo.png" {
		t.Fatalf("unexpected public URL: %q", result.PublicURL)
	}
}

func TestGetObjectReturnsSDKBody(t *testing.T) {
	client := &fakeS3Client{}
	adapter := NewAdapterWithOptions(nil, WithClientFactory(func(context.Context, oss.ProviderConfig) (S3API, error) {
		return client, nil
	}))

	result, err := adapter.GetObject(t.Context(), testProviderConfig(), oss.GetObjectRequest{Key: "settings/site-logo.png"})
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read result body: %v", err)
	}

	if aws.ToString(client.getInput.Key) != "public/settings/site-logo.png" {
		t.Fatalf("unexpected object key: %q", aws.ToString(client.getInput.Key))
	}
	if string(body) != "image-bytes" || result.ContentType != "image/png" || result.Size != int64(len("image-bytes")) || result.ETag != "etag-get" {
		t.Fatalf("unexpected get result: body=%q result=%#v", string(body), result)
	}
}

func TestGetObjectMapsNoSuchKeyToPermanentProviderError(t *testing.T) {
	client := &fakeS3Client{err: &types.NoSuchKey{}}
	adapter := NewAdapterWithOptions(nil, WithClientFactory(func(context.Context, oss.ProviderConfig) (S3API, error) {
		return client, nil
	}))

	_, err := adapter.GetObject(t.Context(), testProviderConfig(), oss.GetObjectRequest{Key: "missing.png"})
	if err == nil {
		t.Fatalf("expected missing object error")
	}
	providerErr, ok := providererror.From(err)
	if !ok {
		t.Fatalf("expected provider error, got %T %v", err, err)
	}
	if providerErr.Category != providererror.CategoryPermanent || providerErr.Retryable {
		t.Fatalf("unexpected provider error: %#v", providerErr)
	}
}

func TestDeleteObjectMapsMissingObjectToDeletedFalse(t *testing.T) {
	client := &fakeS3Client{err: &types.NoSuchKey{}}
	adapter := NewAdapterWithOptions(nil, WithClientFactory(func(context.Context, oss.ProviderConfig) (S3API, error) {
		return client, nil
	}))

	result, err := adapter.DeleteObject(t.Context(), testProviderConfig(), oss.DeleteObjectRequest{Key: "missing.png"})
	if err != nil {
		t.Fatalf("delete missing object: %v", err)
	}
	if result.Deleted {
		t.Fatalf("expected deleted=false for missing object, got %#v", result)
	}
}

func TestPresignObjectUsesSDKPresigner(t *testing.T) {
	presigner := &fakePresignClient{}
	adapter := NewAdapterWithOptions(nil, WithPresignFactory(func(context.Context, oss.ProviderConfig) (PresignAPI, error) {
		return presigner, nil
	}))
	adapter.now = func() time.Time {
		return time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	}

	result, err := adapter.PresignObject(t.Context(), testProviderConfig(), oss.PresignObjectRequest{
		Key:       "settings/site-logo.png",
		Method:    http.MethodPut,
		ExpiresIn: 15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("presign object: %v", err)
	}

	if aws.ToString(presigner.putInput.Key) != "public/settings/site-logo.png" {
		t.Fatalf("unexpected presign key: %q", aws.ToString(presigner.putInput.Key))
	}
	if presigner.expires != 15*time.Minute {
		t.Fatalf("unexpected presign expiry: %s", presigner.expires)
	}
	if result.URL != "https://signed.example.com/put" || !result.ExpiresAt.Equal(time.Date(2026, 6, 8, 12, 15, 0, 0, time.UTC)) {
		t.Fatalf("unexpected presign result: %#v", result)
	}
}

func TestUsePathStyleDefaultsByProvider(t *testing.T) {
	r2 := testProviderConfig()
	r2.ProviderCode = "cloudflare_r2"
	if !usePathStyle(r2) {
		t.Fatalf("expected Cloudflare R2 to default to path-style addressing")
	}

	aliyun := testProviderConfig()
	aliyun.ProviderCode = "aliyun"
	if usePathStyle(aliyun) {
		t.Fatalf("expected Aliyun OSS to default to virtual-hosted addressing")
	}

	override := true
	aliyun.UsePathStyle = &override
	if !usePathStyle(aliyun) {
		t.Fatalf("expected explicit use_path_style to override provider default")
	}
}

func presignExpires(options ...func(*s3.PresignOptions)) time.Duration {
	var opts s3.PresignOptions
	for _, option := range options {
		option(&opts)
	}
	return opts.Expires
}

func testProviderConfig() oss.ProviderConfig {
	return oss.ProviderConfig{
		ProviderCode:    "cloudflare_r2",
		EndpointURL:     "https://r2.example.com",
		Bucket:          "assets",
		Region:          "auto",
		PublicBaseURL:   "https://assets.example.com",
		KeyPrefix:       "public",
		AccessKeyID:     "ak",
		SecretAccessKey: "sk",
	}
}
