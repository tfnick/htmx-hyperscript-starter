package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/integrations/providererror"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type Adapter struct {
	rootDir string
}

func NewAdapter(rootDir string) *Adapter {
	return &Adapter{rootDir: rootDir}
}

func (a *Adapter) PutObject(_ context.Context, cfg oss.ProviderConfig, req oss.PutObjectRequest) (oss.PutObjectResult, error) {
	if err := a.validateConfig(); err != nil {
		return oss.PutObjectResult{}, err
	}
	key, target, err := a.objectPath(req.Key)
	if err != nil {
		return oss.PutObjectResult{}, err
	}
	if req.Body == nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryValidation, false, "OSS object body is required", nil)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to create OSS object directory", err)
	}

	file, err := os.Create(target)
	if err != nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to create OSS object", err)
	}
	written, copyErr := io.Copy(file, req.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to write OSS object", copyErr)
	}
	if closeErr != nil {
		return oss.PutObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to close OSS object", closeErr)
	}

	return oss.PutObjectResult{
		Key:       key,
		Size:      written,
		PublicURL: localPublicURL(cfg.PublicBaseURL, key),
	}, nil
}

func (a *Adapter) GetObject(_ context.Context, _ oss.ProviderConfig, req oss.GetObjectRequest) (oss.GetObjectResult, error) {
	if err := a.validateConfig(); err != nil {
		return oss.GetObjectResult{}, err
	}
	key, target, err := a.objectPath(req.Key)
	if err != nil {
		return oss.GetObjectResult{}, err
	}

	file, err := os.Open(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return oss.GetObjectResult{}, providererror.New(providererror.CategoryPermanent, false, "OSS object not found", err)
		}
		return oss.GetObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to open OSS object", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return oss.GetObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to stat OSS object", err)
	}

	return oss.GetObjectResult{
		Key:         key,
		Body:        file,
		ContentType: contentTypeForKey(key),
		Size:        info.Size(),
	}, nil
}

func (a *Adapter) DeleteObject(_ context.Context, _ oss.ProviderConfig, req oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	if err := a.validateConfig(); err != nil {
		return oss.DeleteObjectResult{}, err
	}
	_, target, err := a.objectPath(req.Key)
	if err != nil {
		return oss.DeleteObjectResult{}, err
	}
	if err := os.Remove(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return oss.DeleteObjectResult{Deleted: false}, nil
		}
		return oss.DeleteObjectResult{}, providererror.New(providererror.CategoryTemporary, true, "failed to delete OSS object", err)
	}
	return oss.DeleteObjectResult{Deleted: true}, nil
}

func (a *Adapter) PresignObject(_ context.Context, cfg oss.ProviderConfig, req oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	if err := a.validateConfig(); err != nil {
		return oss.PresignObjectResult{}, err
	}
	if _, _, err := a.objectPath(req.Key); err != nil {
		return oss.PresignObjectResult{}, err
	}
	expiresIn := req.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = time.Hour
	}
	return oss.PresignObjectResult{
		URL:       localPublicURL(cfg.PublicBaseURL, req.Key),
		ExpiresAt: time.Now().UTC().Add(expiresIn),
	}, nil
}

func (a *Adapter) validateConfig() error {
	if strings.TrimSpace(a.rootDir) == "" {
		return providererror.New(providererror.CategoryValidation, false, "OSS local root is required", nil)
	}
	return nil
}

func (a *Adapter) objectPath(key string) (string, string, error) {
	key = cleanObjectKey(key)
	if key == "" {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS object key is required", nil)
	}
	if strings.HasPrefix(key, "../") || key == ".." {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS object key is invalid", fmt.Errorf("key escapes root"))
	}

	root, err := filepath.Abs(a.rootDir)
	if err != nil {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS local root is invalid", err)
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(key)))
	if err != nil {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS object key is invalid", err)
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", "", providererror.New(providererror.CategoryValidation, false, "OSS object key is invalid", fmt.Errorf("key escapes root"))
	}
	return key, target, nil
}

func cleanObjectKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	key = filepath.ToSlash(filepath.Clean(key))
	if key == "." {
		return ""
	}
	return key
}

func localPublicURL(baseURL string, key string) string {
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

func contentTypeForKey(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
