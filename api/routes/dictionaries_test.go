package routes_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/routes"
)

func TestGetDictionariesUsesInternalSuccessEnvelope(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dictionaries?types=product_category", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetDictionaries(c); err != nil {
		t.Fatalf("get dictionaries: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected success envelope, got %s", body)
	}
	if !strings.Contains(body, `"data":{"dictionaries"`) {
		t.Fatalf("expected dictionaries under data, got %s", body)
	}
}

func TestGetDictionariesValidationErrorUsesInternalErrorEnvelope(t *testing.T) {
	router := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dictionaries", nil)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)

	if err := routes.GetDictionaries(c); err != nil {
		t.Fatalf("get dictionaries: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	expected := `{"success":false,"error":{"code":"validation","message":"types is required"}}`
	if body != expected {
		t.Fatalf("expected %s, got %s", expected, body)
	}
}

func TestDictionaryManagementRoutesReturnDTOs(t *testing.T) {
	setupRouteTestDBs(t)

	router := echo.New()
	createTypeBody := bytes.NewBufferString(`{
		"type_key":"route_status",
		"name":"Route status",
		"enabled":true,
		"description":"Route dictionary"
	}`)
	createTypeReq := httptest.NewRequest(http.MethodPost, "/api/dictionary/types", createTypeBody)
	createTypeReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createTypeRec := httptest.NewRecorder()
	createTypeCtx := router.NewContext(createTypeReq, createTypeRec)

	if err := routes.CreateDictionaryType(createTypeCtx); err != nil {
		t.Fatalf("create dictionary type: %v", err)
	}
	if createTypeRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, createTypeRec.Code, createTypeRec.Body.String())
	}

	var typeEnvelope struct {
		Success bool                          `json:"success"`
		Data    routes.DictionaryTypeResponse `json:"data"`
	}
	if err := json.Unmarshal(createTypeRec.Body.Bytes(), &typeEnvelope); err != nil {
		t.Fatalf("decode type response: %v", err)
	}
	if !typeEnvelope.Success || typeEnvelope.Data.ID == "" || typeEnvelope.Data.TypeKey != "route_status" {
		t.Fatalf("unexpected type response: %s", createTypeRec.Body.String())
	}

	createValueBody := bytes.NewBufferString(`{
		"value_code":"pending",
		"label":"Pending",
		"sort_order":10,
		"enabled":true,
		"description":"Waiting"
	}`)
	createValueReq := httptest.NewRequest(http.MethodPost, "/api/dictionary/types/type-1/values", createValueBody)
	createValueReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createValueRec := httptest.NewRecorder()
	createValueCtx := router.NewContext(createValueReq, createValueRec)
	createValueCtx.SetParamNames("type_id")
	createValueCtx.SetParamValues(typeEnvelope.Data.ID)

	if err := routes.CreateDictionaryValue(createValueCtx); err != nil {
		t.Fatalf("create dictionary value: %v", err)
	}
	if createValueRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusCreated, createValueRec.Code, createValueRec.Body.String())
	}

	var valueEnvelope struct {
		Success bool                           `json:"success"`
		Data    routes.DictionaryValueResponse `json:"data"`
	}
	if err := json.Unmarshal(createValueRec.Body.Bytes(), &valueEnvelope); err != nil {
		t.Fatalf("decode value response: %v", err)
	}
	if !valueEnvelope.Success || valueEnvelope.Data.DictionaryTypeID != typeEnvelope.Data.ID || valueEnvelope.Data.TypeKey != "route_status" {
		t.Fatalf("unexpected value response: %s", createValueRec.Body.String())
	}
}

func TestSetDictionaryValueEnabledReturnsUpdatedDTO(t *testing.T) {
	setupRouteTestDBs(t)
	typeID, valueID := seedRouteDictionary(t)

	router := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/dictionary/values/value-1/enabled", bytes.NewBufferString(`{"enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := router.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(valueID)

	if err := routes.SetDictionaryValueEnabled(c); err != nil {
		t.Fatalf("set dictionary value enabled: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool                           `json:"success"`
		Data    routes.DictionaryValueResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Success || envelope.Data.Enabled || envelope.Data.DictionaryTypeID != typeID {
		t.Fatalf("expected disabled value response, got %s", rec.Body.String())
	}
}

func seedRouteDictionary(t *testing.T) (string, string) {
	t.Helper()

	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO dictionary_types (id, type_key, name, enabled, description)
		VALUES (?, 'route_type', 'Route type', 1, 'Route test type')
	`, "route-dictionary-type"); err != nil {
		t.Fatalf("insert dictionary type: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO dictionary_values (id, dictionary_type_id, value_code, label, sort_order, enabled, description)
		VALUES (?, ?, 'active', 'Active', 10, 1, 'Route test value')
	`, "route-dictionary-value", "route-dictionary-type"); err != nil {
		t.Fatalf("insert dictionary value: %v", err)
	}
	return "route-dictionary-type", "route-dictionary-value"
}

func setupRouteTestDBs(t *testing.T) {
	t.Helper()

	previous := db.DefaultManager
	manager := db.NewDBManager()
	db.DefaultManager = manager

	dir := t.TempDir()
	t.Cleanup(func() {
		_ = manager.Close()
		db.DefaultManager = previous
	})

	if err := manager.Open("app", "sqlite", filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := manager.Open("shared", "sqlite", filepath.Join(dir, "shared.db")); err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	if err := manager.AutoMigrate("shared"); err != nil {
		t.Fatalf("migrate shared db: %v", err)
	}
}
