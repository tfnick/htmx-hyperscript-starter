ALTER TABLE user_registration_profiles ADD COLUMN utm_source TEXT NOT NULL DEFAULT '';
ALTER TABLE user_registration_profiles ADD COLUMN utm_medium TEXT NOT NULL DEFAULT '';
ALTER TABLE user_registration_profiles ADD COLUMN utm_campaign TEXT NOT NULL DEFAULT '';
