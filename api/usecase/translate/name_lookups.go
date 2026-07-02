package translate

import (
	"context"

	"github.com/tfnick/go-svelte-starter/api/framework/data/namelookup"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	UserDisplayName    namelookup.Key = "user.display_name"
	ProductDisplayName namelookup.Key = "product.display_name"
)

var appNameLookups = namelookup.NewRegistry(
	namelookup.Resource(UserDisplayName, models.GetUserDisplayNamesByIDs),
	namelookup.Resource(ProductDisplayName, models.GetProductDisplayNamesByIDs),
)

func Resolve(ctx context.Context, collect func(*namelookup.Batch)) (namelookup.Result, error) {
	return appNameLookups.Resolve(ctx, collect)
}
