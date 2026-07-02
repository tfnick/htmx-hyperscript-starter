package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
)

type resolvedOSSProvider struct {
	Config  oss.ProviderConfig
	Adapter oss.Adapter
}

func primaryOSSProvider(ctx fwusecase.Context, unavailableMessage string) (resolvedOSSProvider, error) {
	channel, err := cache.Cached(ctx.Std(), "config.oss", "primary_enabled", 5*time.Minute, func() (models.IntegrationChannelConfig, error) {
		return models.GetEnabledPrimaryOSSChannelConfig(ctx.Std())
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return resolvedOSSProvider{}, fwusecase.E(fwusecase.CodeValidation, unavailableMessage, err)
		}
		return resolvedOSSProvider{}, fwusecase.E(fwusecase.CodeInternal, "failed to load primary OSS provider", err)
	}
	return ossProviderFromChannel(channel, unavailableMessage)
}

func ossProviderFromMetadata(ctx fwusecase.Context, channelCode string, adapterKey string, unavailableMessage string) (resolvedOSSProvider, error) {
	channelCode = strings.TrimSpace(channelCode)
	adapterKey = strings.TrimSpace(adapterKey)
	if channelCode == "" || adapterKey == "" {
		return primaryOSSProvider(ctx, unavailableMessage)
	}

	channel, err := models.GetOSSChannelConfigByCodeAndAdapter(ctx.Std(), channelCode, adapterKey)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return resolvedOSSProvider{}, fwusecase.E(fwusecase.CodeInternal, unavailableMessage, err)
		}
		return resolvedOSSProvider{}, fwusecase.E(fwusecase.CodeInternal, "failed to load OSS provider", err)
	}
	return ossProviderFromChannel(channel, unavailableMessage)
}

func ossProviderFromChannel(channel models.IntegrationChannelConfig, unavailableMessage string) (resolvedOSSProvider, error) {
	provider, err := siteLogoProviderFromChannel(channel)
	if err != nil {
		return resolvedOSSProvider{}, err
	}

	adapter, ok := registeredOSSAdapter(provider.Config.AdapterKey)
	if !ok {
		cause := fmt.Errorf("OSS adapter not registered: %s", provider.Config.AdapterKey)
		return resolvedOSSProvider{}, fwusecase.E(fwusecase.CodeInternal, unavailableMessage, cause)
	}

	return resolvedOSSProvider{
		Config:  provider.Config,
		Adapter: adapter,
	}, nil
}
