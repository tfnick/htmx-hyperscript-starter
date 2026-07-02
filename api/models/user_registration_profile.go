package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

type UserRegistrationProfile struct {
	ID                    string `json:"id" db:"id"`
	UserID                string `json:"user_id" db:"user_id"`
	RegistrationIP        string `json:"registration_ip" db:"registration_ip"`
	RegistrationCountry   string `json:"registration_country" db:"registration_country"`
	RegistrationRegion    string `json:"registration_region" db:"registration_region"`
	RegistrationUserAgent string `json:"registration_user_agent" db:"registration_user_agent"`
	UtmSource             string `json:"utm_source" db:"utm_source"`
	UtmMedium             string `json:"utm_medium" db:"utm_medium"`
	UtmCampaign           string `json:"utm_campaign" db:"utm_campaign"`
	CreatedAt             string `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt             string `json:"updated_at,omitempty" db:"updated_at"`
}

func CreateUserRegistrationProfile(ctx context.Context, profile *UserRegistrationProfile) error {
	if profile == nil {
		return fmt.Errorf("registration profile is required")
	}
	if strings.TrimSpace(profile.UserID) == "" {
		return fmt.Errorf("registration profile user ID is required")
	}
	if profile.ID == "" {
		profile.ID = uuid.Must(uuid.NewV7()).String()
	}
	profile.UserID = strings.TrimSpace(profile.UserID)
	profile.RegistrationIP = strings.TrimSpace(profile.RegistrationIP)
	profile.RegistrationCountry = strings.TrimSpace(profile.RegistrationCountry)
	profile.RegistrationRegion = strings.TrimSpace(profile.RegistrationRegion)
	profile.RegistrationUserAgent = strings.TrimSpace(profile.RegistrationUserAgent)
	profile.UtmSource = strings.TrimSpace(profile.UtmSource)
	profile.UtmMedium = strings.TrimSpace(profile.UtmMedium)
	profile.UtmCampaign = strings.TrimSpace(profile.UtmCampaign)
	profile.CreatedAt = timefmt.NowSQLiteDateTime()
	profile.UpdatedAt = profile.CreatedAt

	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	sql := `
		INSERT INTO user_registration_profiles (
			id, user_id, registration_ip, registration_country, registration_region, registration_user_agent, utm_source, utm_medium, utm_campaign, created_at, updated_at
		) VALUES (
			:id, :user_id, :registration_ip, :registration_country, :registration_region, :registration_user_agent, :utm_source, :utm_medium, :utm_campaign, :created_at, :updated_at
		)
	`
	if _, err := eng.ExecNamed(sql, profile); err != nil {
		return fmt.Errorf("create user registration profile failed: %w", err)
	}
	return nil
}

func GetUserRegistrationProfileByUserID(ctx context.Context, userID string) (*UserRegistrationProfile, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var profile UserRegistrationProfile
	if err := eng.GetP(&profile, `SELECT * FROM user_registration_profiles WHERE user_id = ?`, strings.TrimSpace(userID)); err != nil {
		return nil, fmt.Errorf("get user registration profile failed: %w", err)
	}
	return &profile, nil
}
