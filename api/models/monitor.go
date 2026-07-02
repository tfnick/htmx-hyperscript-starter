package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const MonitorSettingsIDDefault = "default"

type MonitorSettings struct {
	ID                    string `db:"id"`
	SampleIntervalSeconds int    `db:"sample_interval_seconds"`
	LatencyProbeEnabled   int    `db:"latency_probe_enabled"`
	LatencyProbeURL       string `db:"latency_probe_url"`
	LatencyProbeTimeoutMS int    `db:"latency_probe_timeout_ms"`
	AlertBotChannelID     string `db:"alert_bot_channel_id"`
	DailyAlertLimit       int    `db:"daily_alert_limit"`
	CreatedAt             string `db:"created_at"`
	UpdatedAt             string `db:"updated_at"`
}

type MonitorAlertRule struct {
	ID               string  `db:"id"`
	MetricKey        string  `db:"metric_key"`
	Enabled          int     `db:"enabled"`
	ThresholdValue   float64 `db:"threshold_value"`
	SustainedSamples int     `db:"sustained_samples"`
	CreatedAt        string  `db:"created_at"`
	UpdatedAt        string  `db:"updated_at"`
}

type SaveMonitorSettingsCmd struct {
	SampleIntervalSeconds int
	LatencyProbeEnabled   bool
	LatencyProbeURL       string
	LatencyProbeTimeoutMS int
	AlertBotChannelID     string
	DailyAlertLimit       int
}

type SaveMonitorAlertRuleCmd struct {
	MetricKey        string
	Enabled          bool
	ThresholdValue   float64
	SustainedSamples int
}

func EnsureMonitorDefaults(ctx context.Context) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecP(`INSERT INTO monitor_settings (id) VALUES (?) ON CONFLICT(id) DO NOTHING`, MonitorSettingsIDDefault); err != nil {
		return fmt.Errorf("ensure monitor settings failed: %w", err)
	}
	for _, rule := range defaultMonitorRules() {
		if _, err := d.ExecP(`
			INSERT INTO monitor_alert_rules (id, metric_key, enabled, threshold_value, sustained_samples)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(metric_key) DO NOTHING
		`, rule.ID, rule.MetricKey, rule.Enabled, rule.ThresholdValue, rule.SustainedSamples); err != nil {
			return fmt.Errorf("ensure monitor alert rule failed: %w", err)
		}
	}
	return nil
}

func GetMonitorSettings(ctx context.Context) (MonitorSettings, error) {
	if err := EnsureMonitorDefaults(ctx); err != nil {
		return MonitorSettings{}, err
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return MonitorSettings{}, fmt.Errorf("database unavailable: %w", err)
	}
	var settings MonitorSettings
	if err := d.GetP(&settings, `
		SELECT id, sample_interval_seconds, latency_probe_enabled, latency_probe_url,
			latency_probe_timeout_ms, alert_bot_channel_id, daily_alert_limit, created_at, updated_at
		FROM monitor_settings
		WHERE id = ?
		LIMIT 1
	`, MonitorSettingsIDDefault); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MonitorSettings{}, fmt.Errorf("monitor settings not found: %w", modelerror.ErrNotFound)
		}
		return MonitorSettings{}, fmt.Errorf("load monitor settings failed: %w", err)
	}
	return settings, nil
}

func SaveMonitorSettings(ctx context.Context, cmd SaveMonitorSettingsCmd) (MonitorSettings, error) {
	if err := EnsureMonitorDefaults(ctx); err != nil {
		return MonitorSettings{}, err
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return MonitorSettings{}, fmt.Errorf("database unavailable: %w", err)
	}
	_, err = d.ExecP(`
		UPDATE monitor_settings SET
			sample_interval_seconds = ?,
			latency_probe_enabled = ?,
			latency_probe_url = ?,
			latency_probe_timeout_ms = ?,
			alert_bot_channel_id = ?,
			daily_alert_limit = ?,
			updated_at = ?
		WHERE id = ?
	`, cmd.SampleIntervalSeconds, boolToInt(cmd.LatencyProbeEnabled), cmd.LatencyProbeURL,
		cmd.LatencyProbeTimeoutMS, cmd.AlertBotChannelID, cmd.DailyAlertLimit,
		timefmt.NowSQLiteDateTime(), MonitorSettingsIDDefault)
	if err != nil {
		return MonitorSettings{}, fmt.Errorf("save monitor settings failed: %w", err)
	}
	return GetMonitorSettings(ctx)
}

func ListMonitorAlertRules(ctx context.Context) ([]MonitorAlertRule, error) {
	if err := EnsureMonitorDefaults(ctx); err != nil {
		return nil, err
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	var rules []MonitorAlertRule
	if err := d.SelectP(&rules, `
		SELECT id, metric_key, enabled, threshold_value, sustained_samples, created_at, updated_at
		FROM monitor_alert_rules
		ORDER BY CASE metric_key
			WHEN 'cpu' THEN 1
			WHEN 'memory' THEN 2
			WHEN 'disk' THEN 3
			WHEN 'latency' THEN 4
			ELSE 99
		END
	`); err != nil {
		return nil, fmt.Errorf("list monitor alert rules failed: %w", err)
	}
	return rules, nil
}

func SaveMonitorAlertRules(ctx context.Context, cmds []SaveMonitorAlertRuleCmd) ([]MonitorAlertRule, error) {
	if err := EnsureMonitorDefaults(ctx); err != nil {
		return nil, err
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	for _, cmd := range cmds {
		result, err := d.ExecP(`
			UPDATE monitor_alert_rules SET
				enabled = ?,
				threshold_value = ?,
				sustained_samples = ?,
				updated_at = ?
			WHERE metric_key = ?
		`, boolToInt(cmd.Enabled), cmd.ThresholdValue, cmd.SustainedSamples, timefmt.NowSQLiteDateTime(), cmd.MetricKey)
		if err != nil {
			return nil, fmt.Errorf("save monitor alert rule failed: %w", err)
		}
		if err := requireRowsAffected(result, "monitor alert rule not found"); err != nil {
			return nil, err
		}
	}
	return ListMonitorAlertRules(ctx)
}

func defaultMonitorRules() []MonitorAlertRule {
	return []MonitorAlertRule{
		{ID: "monitor-rule-cpu", MetricKey: "cpu", Enabled: 0, ThresholdValue: 90, SustainedSamples: 3},
		{ID: "monitor-rule-memory", MetricKey: "memory", Enabled: 0, ThresholdValue: 90, SustainedSamples: 3},
		{ID: "monitor-rule-disk", MetricKey: "disk", Enabled: 0, ThresholdValue: 90, SustainedSamples: 3},
		{ID: "monitor-rule-latency", MetricKey: "latency", Enabled: 0, ThresholdValue: 1000, SustainedSamples: 3},
	}
}
