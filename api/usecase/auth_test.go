package usecase_test

import (
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type fakeRegistrationGeoResolver struct {
	geo usecase.RegistrationGeo
	err error
}

func (r fakeRegistrationGeoResolver) ResolveRegistrationGeo(ctx fwusecase.Context, ip string) (usecase.RegistrationGeo, error) {
	return r.geo, r.err
}

func TestRegisterPersistsRegistrationProfile(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	usecase.RegisterRegistrationGeoResolver(fakeRegistrationGeoResolver{
		geo: usecase.RegistrationGeo{
			Country: "United States",
			Region:  "California",
		},
	})
	t.Cleanup(func() {
		usecase.RegisterRegistrationGeoResolver(nil)
	})

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	auth, err := usecase.Register(ctx, usecase.RegisterCmd{
		Name:     "Ada",
		Email:    "ada@example.com",
		Password: "secret123",
		Context: usecase.RegistrationContext{
			IP:        "203.0.113.10",
			UserAgent: "Mozilla/5.0 registration-test",
		},
		UtmSource:   "xiaohongshu",
		UtmMedium:   "social",
		UtmCampaign: "launch",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	profile, err := models.GetUserRegistrationProfileByUserID(t.Context(), auth.User.ID)
	if err != nil {
		t.Fatalf("get registration profile: %v", err)
	}
	if profile.RegistrationIP != "203.0.113.10" || profile.RegistrationUserAgent != "Mozilla/5.0 registration-test" {
		t.Fatalf("unexpected registration request metadata: %#v", profile)
	}
	if profile.RegistrationCountry != "United States" || profile.RegistrationRegion != "California" {
		t.Fatalf("unexpected registration geo metadata: %#v", profile)
	}
	if profile.UtmSource != "xiaohongshu" || profile.UtmMedium != "social" || profile.UtmCampaign != "launch" {
		t.Fatalf("unexpected registration UTM metadata: %#v", profile)
	}
}

func TestRegisterKeepsSuccessWhenRegistrationGeoResolverFails(t *testing.T) {
	setupUsecaseOrderTxDB(t)
	usecase.RegisterRegistrationGeoResolver(fakeRegistrationGeoResolver{err: assertErr("geo unavailable")})
	t.Cleanup(func() {
		usecase.RegisterRegistrationGeoResolver(nil)
	})

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	auth, err := usecase.Register(ctx, usecase.RegisterCmd{
		Name:     "Grace",
		Email:    "grace@example.com",
		Password: "secret123",
		Context: usecase.RegistrationContext{
			IP:        "198.51.100.24",
			UserAgent: "resolver-failure-test",
		},
	})
	if err != nil {
		t.Fatalf("register should succeed when geo resolver fails: %v", err)
	}

	profile, err := models.GetUserRegistrationProfileByUserID(t.Context(), auth.User.ID)
	if err != nil {
		t.Fatalf("get registration profile: %v", err)
	}
	if profile.RegistrationIP != "198.51.100.24" || profile.RegistrationUserAgent != "resolver-failure-test" {
		t.Fatalf("unexpected registration request metadata: %#v", profile)
	}
	if profile.RegistrationCountry != "" || profile.RegistrationRegion != "" {
		t.Fatalf("expected empty geo metadata after resolver failure, got %#v", profile)
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
