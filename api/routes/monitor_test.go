package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

func TestMonitorRoutesReturnAndSaveConfig(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/monitor", nil)
	getRec := httptest.NewRecorder()
	getCtx := router.NewContext(getReq, getRec)
	if err := routes.GetMonitor(getCtx); err != nil {
		t.Fatalf("get monitor: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, getRec.Code, getRec.Body.String())
	}
	var getEnvelope struct {
		Success bool                   `json:"success"`
		Data    routes.MonitorResponse `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getEnvelope); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if !getEnvelope.Success || len(getEnvelope.Data.Rules) != 4 || getEnvelope.Data.Settings.SampleIntervalSeconds != 10 {
		t.Fatalf("unexpected get response: %s", getRec.Body.String())
	}

	saveReq := httptest.NewRequest(http.MethodPut, "/api/admin/monitor/config", strings.NewReader(`{
		"sample_interval_seconds":15,
		"latency_probe_enabled":true,
		"latency_probe_url":"https://www.google.com",
		"latency_probe_timeout_ms":1500,
		"daily_alert_limit":1,
		"rules":[{"metric_key":"cpu","enabled":true,"threshold_value":80,"sustained_samples":2}]
	}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	saveCtx := router.NewContext(saveReq, saveRec)
	if err := routes.SaveMonitorConfig(saveCtx); err != nil {
		t.Fatalf("save monitor: %v", err)
	}
	if saveRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, saveRec.Code, saveRec.Body.String())
	}
	var saveEnvelope struct {
		Success bool                   `json:"success"`
		Data    routes.MonitorResponse `json:"data"`
	}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saveEnvelope); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if !saveEnvelope.Success || saveEnvelope.Data.Settings.SampleIntervalSeconds != 15 || !saveEnvelope.Data.Rules[0].Enabled {
		t.Fatalf("unexpected save response: %s", saveRec.Body.String())
	}
}
