package local_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/providers/oss/local"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

func TestAdapterStoresAndReadsObjectInsideRoot(t *testing.T) {
	adapter := local.NewAdapter(t.TempDir())
	payload := []byte("logo")

	put, err := adapter.PutObject(t.Context(), oss.ProviderConfig{
		PublicBaseURL: "/assets",
	}, oss.PutObjectRequest{
		Key:         "settings/logo.png",
		Body:        bytes.NewReader(payload),
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("put object: %v", err)
	}
	if put.Key != "settings/logo.png" || put.PublicURL != "/assets/settings/logo.png" {
		t.Fatalf("unexpected put result: %#v", put)
	}

	got, err := adapter.GetObject(t.Context(), oss.ProviderConfig{}, oss.GetObjectRequest{Key: put.Key})
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer got.Body.Close()
	read, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if !bytes.Equal(read, payload) || got.ContentType != "image/png" {
		t.Fatalf("unexpected object result: %#v body=%q", got, string(read))
	}
}

func TestAdapterRejectsEscapingObjectKey(t *testing.T) {
	adapter := local.NewAdapter(t.TempDir())
	_, err := adapter.PutObject(t.Context(), oss.ProviderConfig{}, oss.PutObjectRequest{
		Key:  "../logo.png",
		Body: strings.NewReader("logo"),
	})
	if err == nil {
		t.Fatalf("expected escaping key to fail")
	}
}
