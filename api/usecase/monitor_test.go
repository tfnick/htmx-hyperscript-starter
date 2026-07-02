package usecase

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/tfnick/go-svelte-starter/api/models"
)

func TestMonitorEvaluateAlertsSkipsWhenAlertBotIsEmpty(t *testing.T) {
	sampler := &monitorSampler{
		nodeID:      "node-1",
		hostname:    "host-1",
		alertCounts: map[string]int{},
		samples: []monitorSample{{
			SampledAt: time.Now().UTC(),
			CPU:       MonitorCPUCo{UsagePercent: 99},
		}},
	}

	sampler.evaluateAlerts(t.Context(), models.MonitorSettings{
		DailyAlertLimit:   1,
		AlertBotChannelID: "",
	}, []models.MonitorAlertRule{{
		MetricKey:        MonitorMetricCPU,
		Enabled:          1,
		ThresholdValue:   90,
		SustainedSamples: 1,
	}})

	if len(sampler.alertCounts) != 0 {
		t.Fatalf("expected no alert count when alert bot is empty, got %#v", sampler.alertCounts)
	}
}

func TestMonitorStealPercentFromTimes(t *testing.T) {
	before := []cpu.TimesStat{{User: 40, System: 10, Idle: 49, Steal: 1}}
	after := []cpu.TimesStat{{User: 80, System: 20, Idle: 89, Steal: 11}}

	got := monitorStealPercent(before, after)
	if got != 10 {
		t.Fatalf("expected steal percent 10, got %.2f", got)
	}
}

func TestMonitorStealPercentReturnsZeroForInvalidDelta(t *testing.T) {
	value := []cpu.TimesStat{{User: 40, System: 10, Idle: 49, Steal: 1}}

	if got := monitorStealPercent(value, value); got != 0 {
		t.Fatalf("expected zero steal percent for unchanged times, got %.2f", got)
	}
}

func TestMonitorLoadPerCore(t *testing.T) {
	if got := monitorLoadPerCore(3, 2); got != 1.5 {
		t.Fatalf("expected load per core 1.5, got %.2f", got)
	}
	if got := monitorLoadPerCore(3, 0); got != 0 {
		t.Fatalf("expected zero load per core without core count, got %.2f", got)
	}
}
