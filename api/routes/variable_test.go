package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/routes"
	"github.com/tfnick/sqlx"
)

func TestListVariablesReturnsDTOs(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteVariable(t, appDB)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/variables", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.ListVariables(c); err != nil {
		t.Fatalf("list variables: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                      `json:"success"`
		Data    []routes.VariableResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || len(envelope.Data) != 1 {
		t.Fatalf("unexpected envelope: %s", rec.Body.String())
	}
	if envelope.Data[0].Key != "route.flag" || envelope.Data[0].ValueJSON != "true" {
		t.Fatalf("unexpected response DTO: %#v", envelope.Data[0])
	}
	if strings.Contains(rec.Body.String(), `"purpose"`) {
		t.Fatalf("variable DTO must not expose purpose: %s", rec.Body.String())
	}
}

func TestCreateVariableReturnsCreatedDTO(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	body := bytes.NewBufferString(`{
		"key":"route.limit",
		"name":"Route limit",
		"value_type":"number",
		"value_json":"10",
		"enabled":true,
		"description":"Limit configured from route"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/variables", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.CreateVariable(c); err != nil {
		t.Fatalf("create variable: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                    `json:"success"`
		Data    routes.VariableResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Key != "route.limit" || envelope.Data.ValueJSON != "10" {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestSetVariableEnabledReturnsUpdatedDTO(t *testing.T) {
	setupRouteTestDBs(t)
	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedRouteVariable(t, appDB)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/variables/route-variable/enabled", bytes.NewBufferString(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("route-variable")

	if err := routes.SetVariableEnabled(c); err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var envelope struct {
		Success bool                    `json:"success"`
		Data    routes.VariableResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Enabled {
		t.Fatalf("expected disabled variable response, got %s", rec.Body.String())
	}
}

func seedRouteVariable(t *testing.T, appDB *sqlx.Engine) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO variables (
			id, variable_key, name, value_type, value_json, enabled, description
		) VALUES (?, 'route.flag', 'Route flag', 'boolean', 'true', 1, 'Route test flag')
	`, "route-variable"); err != nil {
		t.Fatalf("insert variable: %v", err)
	}
}
