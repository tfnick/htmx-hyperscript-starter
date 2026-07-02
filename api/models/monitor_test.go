package models_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestMonitorDefaultsAndSave(t *testing.T) {
	setupModelsTestDB(t)

	settings, err := models.GetMonitorSettings(t.Context())
	if err != nil {
		t.Fatalf("get monitor settings: %v", err)
	}
	if settings.SampleIntervalSeconds != 10 || settings.LatencyProbeURL == "" || settings.DailyAlertLimit != 1 {
		t.Fatalf("unexpected default settings: %#v", settings)
	}

	rules, err := models.ListMonitorAlertRules(t.Context())
	if err != nil {
		t.Fatalf("list monitor rules: %v", err)
	}
	if len(rules) != 4 || rules[0].MetricKey != "cpu" || rules[3].MetricKey != "latency" {
		t.Fatalf("unexpected default rules: %#v", rules)
	}

	settings, err = models.SaveMonitorSettings(t.Context(), models.SaveMonitorSettingsCmd{
		SampleIntervalSeconds: 15,
		LatencyProbeEnabled:   false,
		LatencyProbeURL:       "https://status.example.com",
		LatencyProbeTimeoutMS: 1500,
		AlertBotChannelID:     "bot-1",
		DailyAlertLimit:       2,
	})
	if err != nil {
		t.Fatalf("save monitor settings: %v", err)
	}
	if settings.SampleIntervalSeconds != 15 || settings.LatencyProbeEnabled != 0 || settings.AlertBotChannelID != "bot-1" {
		t.Fatalf("unexpected saved settings: %#v", settings)
	}

	rules, err = models.SaveMonitorAlertRules(t.Context(), []models.SaveMonitorAlertRuleCmd{
		{MetricKey: "cpu", Enabled: true, ThresholdValue: 80, SustainedSamples: 2},
	})
	if err != nil {
		t.Fatalf("save monitor rules: %v", err)
	}
	if !boolFromIntForTest(rules[0].Enabled) || rules[0].ThresholdValue != 80 || rules[0].SustainedSamples != 2 {
		t.Fatalf("unexpected saved rule: %#v", rules[0])
	}
}
