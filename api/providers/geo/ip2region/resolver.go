package ip2region

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	ip2regionservice "github.com/lionsoul2014/ip2region/binding/golang/service"
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

const (
	CachePolicyFile        = "file"
	CachePolicyVectorIndex = "vectorIndex"
	CachePolicyContent     = "content"

	DefaultV4XDBPath = "data/geo/ip2region_v4.xdb"
	DefaultV6XDBPath = "data/geo/ip2region_v6.xdb"
	DefaultPoolSize  = 20
)

type Config struct {
	V4XDBPath   string
	V6XDBPath   string
	CachePolicy string
	PoolSize    int
}

type Resolver struct {
	service *ip2regionservice.Ip2Region
}

func NewResolver(config Config) (*Resolver, error) {
	poolSize := config.PoolSize
	if poolSize <= 0 {
		poolSize = DefaultPoolSize
	}

	cachePolicy, err := ip2regionservice.CachePolicyFromName(firstNonEmpty(config.CachePolicy, CachePolicyVectorIndex))
	if err != nil {
		return nil, err
	}

	v4Config, err := versionConfig(strings.TrimSpace(config.V4XDBPath), cachePolicy, poolSize, ip2regionservice.NewV4Config)
	if err != nil {
		return nil, fmt.Errorf("load IPv4 xdb: %w", err)
	}
	v6Config, err := versionConfig(strings.TrimSpace(config.V6XDBPath), cachePolicy, poolSize, ip2regionservice.NewV6Config)
	if err != nil {
		return nil, fmt.Errorf("load IPv6 xdb: %w", err)
	}
	if v4Config == nil && v6Config == nil {
		return nil, fmt.Errorf("at least one ip2region xdb path is required")
	}

	service, err := ip2regionservice.NewIp2Region(v4Config, v6Config)
	if err != nil {
		return nil, err
	}
	return &Resolver{service: service}, nil
}

func ValidateXDBFile(config usecase.GeoIP2RegionXDBValidatorConfig) error {
	version, err := xdb.VersionFromName(config.Version)
	if err != nil {
		return err
	}
	cachePolicy, err := ip2regionservice.CachePolicyFromName(firstNonEmpty(config.CachePolicy, CachePolicyVectorIndex))
	if err != nil {
		return err
	}

	handle, err := os.OpenFile(strings.TrimSpace(config.XDBPath), os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer handle.Close()

	if err := xdb.Verify(handle); err != nil {
		return err
	}
	header, err := xdb.LoadHeader(handle)
	if err != nil {
		return err
	}
	fileVersion, err := xdb.VersionFromHeader(header)
	if err != nil {
		return err
	}
	if fileVersion.Id != version.Id {
		return fmt.Errorf("ip version not match: xdb file %s with ip version=%s, as %s expected", config.XDBPath, fileVersion.Name, version.Name)
	}

	switch cachePolicy {
	case ip2regionservice.VIndexCache:
		if _, err := xdb.LoadVectorIndex(handle); err != nil {
			return err
		}
	case ip2regionservice.BufferCache:
		if _, err := xdb.LoadContent(handle); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) ResolveRegistrationGeo(ctx fwusecase.Context, ip string) (usecase.RegistrationGeo, error) {
	normalizedIP := strings.TrimSpace(ip)
	if normalizedIP == "" {
		return usecase.RegistrationGeo{}, nil
	}
	if parsed := net.ParseIP(normalizedIP); parsed == nil {
		return usecase.RegistrationGeo{}, fmt.Errorf("invalid IP address")
	}

	region, err := r.service.Search(normalizedIP)
	if err != nil {
		return usecase.RegistrationGeo{}, err
	}
	return geoFromRegion(region), nil
}

func (r *Resolver) Close() {
	if r == nil || r.service == nil {
		return
	}
	r.service.CloseTimeout(time.Second)
}

func versionConfig(path string, cachePolicy int, poolSize int, create func(int, string, int) (*ip2regionservice.Config, error)) (*ip2regionservice.Config, error) {
	if path == "" {
		return nil, nil
	}
	return create(cachePolicy, path, poolSize)
}

func geoFromRegion(region string) usecase.RegistrationGeo {
	parts := strings.Split(region, "|")
	if len(parts) < 2 {
		return usecase.RegistrationGeo{}
	}
	country := regionValue(parts[0])
	regionName := regionValue(parts[1])
	return usecase.RegistrationGeo{
		Country: country,
		Region:  regionName,
	}
}

func regionValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "0" {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
