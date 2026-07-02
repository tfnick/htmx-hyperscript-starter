package ip2region

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestGeoFromRegionMapsCountryAndRegion(t *testing.T) {
	geo := geoFromRegion("中国|广东省|深圳市|电信|CN")
	if geo.Country != "中国" || geo.Region != "广东省" {
		t.Fatalf("unexpected geo mapping: %#v", geo)
	}
}

func TestGeoFromRegionDropsZeroValues(t *testing.T) {
	geo := geoFromRegion("0|0|0|内网IP|0")
	if geo.Country != "" || geo.Region != "" {
		t.Fatalf("expected zero placeholders to be empty: %#v", geo)
	}
}

func TestValidateXDBFileDoesNotKeepFileLocked(t *testing.T) {
	source := geoXDBFixturePath(t, "ip2region_v4.xdb")
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "ip2region_v4.xdb")
	copyFileForTest(t, source, target)

	if err := ValidateXDBFile(usecase.GeoIP2RegionXDBValidatorConfig{
		Version:     usecase.GeoXDBVersionV4,
		XDBPath:     target,
		CachePolicy: CachePolicyVectorIndex,
	}); err != nil {
		t.Fatalf("validate xdb file: %v", err)
	}

	renamed := filepath.Join(tmpDir, "ip2region_v4-renamed.xdb")
	if err := os.Rename(target, renamed); err != nil {
		t.Fatalf("validated xdb should not remain locked: %v", err)
	}
	if err := os.Remove(renamed); err != nil {
		t.Fatalf("remove validated xdb: %v", err)
	}
}

func TestValidateXDBFileRejectsMismatchedVersion(t *testing.T) {
	source := geoXDBFixturePath(t, "ip2region_v4.xdb")
	target := filepath.Join(t.TempDir(), "ip2region_v4.xdb")
	copyFileForTest(t, source, target)

	err := ValidateXDBFile(usecase.GeoIP2RegionXDBValidatorConfig{
		Version:     usecase.GeoXDBVersionV6,
		XDBPath:     target,
		CachePolicy: CachePolicyFile,
	})
	if err == nil || !strings.Contains(err.Error(), "ip version not match") {
		t.Fatalf("expected mismatched version error, got %v", err)
	}
}

func geoXDBFixturePath(t *testing.T, filename string) string {
	t.Helper()
	candidates := []string{
		filepath.Join(moduleCacheRoot(t), "github.com", "lionsoul2014", "ip2region@v3.16.0+incompatible", "data", filename),
		filepath.Join("..", "..", "..", "..", "data", "geo", filename),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Skipf("ip2region fixture %s is not available", filename)
	return ""
}

func moduleCacheRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("GOMODCACHE")
	if root != "" {
		return root
	}
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("resolve home dir: %v", err)
		}
		goPath = filepath.Join(home, "go")
	}
	return filepath.Join(goPath, "pkg", "mod")
}

func copyFileForTest(t *testing.T, source string, target string) {
	t.Helper()
	input, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read source file %s: %v", source, err)
	}
	if err := os.WriteFile(target, input, 0644); err != nil {
		t.Fatalf("write target file %s: %v", target, err)
	}
}
