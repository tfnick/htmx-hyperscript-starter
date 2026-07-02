package usecase

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type fakeOSSAdapter struct{}

func (fakeOSSAdapter) PutObject(context.Context, oss.ProviderConfig, oss.PutObjectRequest) (oss.PutObjectResult, error) {
	return oss.PutObjectResult{}, nil
}

func (fakeOSSAdapter) GetObject(context.Context, oss.ProviderConfig, oss.GetObjectRequest) (oss.GetObjectResult, error) {
	return oss.GetObjectResult{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (fakeOSSAdapter) DeleteObject(context.Context, oss.ProviderConfig, oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	return oss.DeleteObjectResult{}, nil
}

func (fakeOSSAdapter) PresignObject(context.Context, oss.ProviderConfig, oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	return oss.PresignObjectResult{}, nil
}

func TestRegisterOSSAdapter(t *testing.T) {
	ossAdaptersMu.Lock()
	ossAdapters = map[string]oss.Adapter{}
	ossAdaptersMu.Unlock()

	if err := RegisterOSSAdapter("", fakeOSSAdapter{}); err == nil {
		t.Fatalf("expected empty adapter key validation")
	}
	if err := RegisterOSSAdapter("oss.fake", nil); err == nil {
		t.Fatalf("expected nil adapter validation")
	}
	if err := RegisterOSSAdapter("oss.fake", fakeOSSAdapter{}); err != nil {
		t.Fatalf("register OSS adapter: %v", err)
	}
	if _, ok := registeredOSSAdapter("oss.fake"); !ok {
		t.Fatalf("expected registered OSS adapter")
	}
}
