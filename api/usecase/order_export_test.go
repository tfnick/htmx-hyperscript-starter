package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/oss"
	"github.com/tfnick/sqlx"
)

type orderExportFakeOSSAdapter struct {
	putKey         string
	putContentType string
	putBody        []byte
	presignKey     string
	presignMethod  string
}

func (a *orderExportFakeOSSAdapter) PutObject(_ context.Context, _ oss.ProviderConfig, req oss.PutObjectRequest) (oss.PutObjectResult, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return oss.PutObjectResult{}, err
	}
	a.putKey = req.Key
	a.putContentType = req.ContentType
	a.putBody = body
	return oss.PutObjectResult{Key: req.Key, Size: int64(len(body))}, nil
}

func (a *orderExportFakeOSSAdapter) GetObject(context.Context, oss.ProviderConfig, oss.GetObjectRequest) (oss.GetObjectResult, error) {
	return oss.GetObjectResult{}, nil
}

func (a *orderExportFakeOSSAdapter) DeleteObject(context.Context, oss.ProviderConfig, oss.DeleteObjectRequest) (oss.DeleteObjectResult, error) {
	return oss.DeleteObjectResult{}, nil
}

func (a *orderExportFakeOSSAdapter) PresignObject(_ context.Context, _ oss.ProviderConfig, req oss.PresignObjectRequest) (oss.PresignObjectResult, error) {
	a.presignKey = req.Key
	a.presignMethod = req.Method
	return oss.PresignObjectResult{
		URL:       "https://download.example.com/" + req.Key,
		ExpiresAt: time.Now().UTC().Add(req.ExpiresIn),
	}, nil
}

func TestEnqueueMyOrdersExcelExportRejectsOversizedResultBeforeQueue(t *testing.T) {
	manager := setupOrderExportUsecaseDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedOrderExportUser(t, appDB, "u1", "Ada")
	seedOrderExportOrder(t, appDB, "o1", "u1", "paid")
	seedOrderExportOrder(t, appDB, "o2", "u1", "paid")
	seedOrderExportOrder(t, appDB, "o3", "u1", "paid")

	adapterKey := "oss.test.order_export.limit." + uuid.Must(uuid.NewV7()).String()
	if err := RegisterOSSAdapter(adapterKey, &orderExportFakeOSSAdapter{}); err != nil {
		t.Fatalf("register OSS adapter: %v", err)
	}
	seedOrderExportPrimaryOSSChannel(t, adapterKey)

	previousMaxRows := orderExportMaxRows
	orderExportMaxRows = 2
	t.Cleanup(func() {
		orderExportMaxRows = previousMaxRows
	})

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("create queue manager: %v", err)
	}
	previousQueue := DefaultQueueManager
	DefaultQueueManager = queueManager
	t.Cleanup(func() {
		DefaultQueueManager = previousQueue
	})

	ctx := authenticatedOrderExportContext(t.Context(), "u1", false)
	_, err = EnqueueMyOrdersExcelExport(ctx, EnqueueMyOrdersExcelExportQry{Status: "paid"})
	if err == nil {
		t.Fatalf("expected oversized export to be rejected")
	}
	if fwusecase.CodeOf(err) != fwusecase.CodeValidation {
		t.Fatalf("expected validation error, got %q: %v", fwusecase.CodeOf(err), err)
	}

	var taskCount int
	if err := appDB.GetP(&taskCount, `SELECT COUNT(*) FROM async_tasks`); err != nil {
		t.Fatalf("count async tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected oversized export not to enqueue a task, got %d", taskCount)
	}
}

func TestOrderExcelExportTaskStreamsUploadsAndDownloadsForOwner(t *testing.T) {
	manager := setupOrderExportUsecaseDB(t)
	appDB, err := manager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	seedOrderExportUser(t, appDB, "admin1", "Admin")
	seedOrderExportUser(t, appDB, "u1", "Ada")
	seedOrderExportUser(t, appDB, "u2", "Byron")
	seedOrderExportProduct(t, appDB, "p1", "Premium Month")
	seedOrderExportOrderWithProduct(t, appDB, "o1", "u1", "p1", "paid")
	seedOrderExportOrderWithProduct(t, appDB, "o2", "u2", "p1", "pending")

	adapter := &orderExportFakeOSSAdapter{}
	adapterKey := "oss.test.order_export.upload." + uuid.Must(uuid.NewV7()).String()
	if err := RegisterOSSAdapter(adapterKey, adapter); err != nil {
		t.Fatalf("register OSS adapter: %v", err)
	}
	seedOrderExportPrimaryOSSChannel(t, adapterKey)

	payload := orderExportTaskPayload{
		Scope:           "admin",
		RequesterUserID: "admin1",
		UserID:          "u1",
		Status:          "paid",
		CreatedAt:       "2026-06-12T00:00:00Z",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	taskID := uuid.Must(uuid.NewV7()).String()
	if err := models.InsertAsyncTask(t.Context(), &models.AsyncTask{
		ID:          taskID,
		UserID:      "admin1",
		TaskType:    HeavyTaskTypeOrdersExcelExport,
		Status:      models.AsyncTaskStatusQueued,
		PayloadJSON: string(payloadJSON),
	}); err != nil {
		t.Fatalf("insert async task: %v", err)
	}

	messageJSON, err := json.Marshal(HeavyTaskMessage{
		TaskID:   taskID,
		TaskType: HeavyTaskTypeOrdersExcelExport,
		UserID:   "admin1",
	})
	if err != nil {
		t.Fatalf("marshal task message: %v", err)
	}
	sub := realtime.SubscribeClient("admin1", "client-order-export-"+taskID)
	defer sub.Close()

	if err := HandleHeavyTaskMessage(t.Context(), messageJSON); err != nil {
		t.Fatalf("handle heavy task message: %v", err)
	}

	select {
	case raw := <-sub.Messages:
		var message struct {
			Type         string `json:"type"`
			Presentation string `json:"presentation"`
			Payload      struct {
				TaskID  string `json:"task_id"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatalf("decode realtime message: %v", err)
		}
		if message.Type != RealtimeMessageTypeAsyncExportTask || message.Presentation != RealtimePresentationToast {
			t.Fatalf("expected async export toast message, got %#v", message)
		}
		if message.Payload.TaskID != taskID || message.Payload.Status != models.AsyncTaskStatusCompleted || message.Payload.Message != "Order export completed" {
			t.Fatalf("unexpected async export payload: %#v", message.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected async export realtime message")
	}

	task, err := models.GetAsyncTaskByID(t.Context(), taskID)
	if err != nil {
		t.Fatalf("get async task: %v", err)
	}
	if task.Status != models.AsyncTaskStatusCompleted {
		t.Fatalf("expected task completed, got %#v", task)
	}
	if adapter.putContentType != orderExportContentType {
		t.Fatalf("expected xlsx content type, got %q", adapter.putContentType)
	}
	if !bytes.HasPrefix(adapter.putBody, []byte("PK")) {
		t.Fatalf("expected xlsx zip payload, got first bytes %v", adapter.putBody[:min(len(adapter.putBody), 4)])
	}

	var result orderExportTaskResult
	if err := json.Unmarshal([]byte(task.ResultJSON), &result); err != nil {
		t.Fatalf("parse result JSON: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected one exported row, got %#v", result)
	}
	if !strings.HasPrefix(result.ObjectKey, "exports/orders/") || result.Filename == "" {
		t.Fatalf("expected export object metadata, got %#v", result)
	}

	var notificationCount int
	if err := appDB.GetP(&notificationCount, `
		SELECT COUNT(*) FROM notifications
		WHERE user_id = ? AND source_type = 'async_task' AND source_id = ?
	`, "admin1", taskID); err != nil {
		t.Fatalf("count export notifications: %v", err)
	}
	if notificationCount != 1 {
		t.Fatalf("expected one export notification, got %d", notificationCount)
	}

	ownerCtx := authenticatedOrderExportContext(t.Context(), "admin1", false)
	download, err := GetMyTaskDownload(ownerCtx, TaskDownloadQry{TaskID: taskID})
	if err != nil {
		t.Fatalf("get task download: %v", err)
	}
	if !strings.Contains(download.URL, result.ObjectKey) || download.Filename != result.Filename {
		t.Fatalf("expected download URL for exported object, got %#v", download)
	}
	if adapter.presignKey != result.ObjectKey || adapter.presignMethod != "GET" {
		t.Fatalf("expected presign for exported object, got key=%q method=%q", adapter.presignKey, adapter.presignMethod)
	}

	otherCtx := authenticatedOrderExportContext(t.Context(), "u1", false)
	_, err = GetMyTaskDownload(otherCtx, TaskDownloadQry{TaskID: taskID})
	if fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden for non-owner download, got %q: %v", fwusecase.CodeOf(err), err)
	}
}

func authenticatedOrderExportContext(ctx context.Context, userID string, admin bool) fwusecase.Context {
	ucCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceInternalAPI)
	ucCtx.Actor = fwusecase.ActorContext{
		Authenticated: true,
		UserID:        userID,
		IsAdmin:       admin,
	}
	return ucCtx
}

func setupOrderExportUsecaseDB(t *testing.T) *db.DBManager {
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
	if err := manager.AutoMigrate("app"); err != nil {
		t.Fatalf("migrate app db: %v", err)
	}
	if err := manager.Open("shared", "sqlite", filepath.Join(dir, "shared.db")); err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	if err := manager.AutoMigrate("shared"); err != nil {
		t.Fatalf("migrate shared db: %v", err)
	}

	return manager
}

func seedOrderExportPrimaryOSSChannel(t *testing.T, adapterKey string) {
	t.Helper()

	credential, err := models.CreateIntegrationCredential(t.Context(), models.CreateIntegrationCredentialCmd{
		CredentialType: "s3_access_key",
		ValueText:      `{"access_key_id":"ak-export","secret_access_key":"sk-export"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create OSS credential: %v", err)
	}
	if _, err := models.CreateIntegrationChannel(t.Context(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioOSS,
		ChannelCode:  "order-export-primary-" + uuid.Must(uuid.NewV7()).String(),
		ProviderCode: "cloudflare_r2",
		AdapterKey:   adapterKey,
		Environment:  "test",
		Enabled:      true,
		Priority:     1,
		CredentialID: credential.ID,
		IsPrimary:    true,
		ConfigJSON:   `{"endpoint_url":"https://r2.example.com","bucket":"exports","region":"auto","key_prefix":"private","use_path_style":true}`,
		MetadataJSON: "{}",
	}); err != nil {
		t.Fatalf("create primary OSS channel: %v", err)
	}
}

func seedOrderExportUser(t *testing.T, appDB *sqlx.Engine, userID string, name string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO users (id, name, email, password_hash, email_verified, is_active, created_at, updated_at)
		VALUES (?, ?, ?, '', 1, 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, userID, name, userID+"@example.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func seedOrderExportProduct(t *testing.T, appDB *sqlx.Engine, productID string, name string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO products (
			id, name, description, price, currency, stock, enabled, creem_product_id,
			billing_type, membership_level, subscription_interval, created_at, updated_at
		) VALUES (?, ?, 'Export test product', 1000, 'USD', 0, 1, 'prod_export', 'subscription', 'premium', 'month', '2026-01-01 00:00:00', '2026-01-01 00:00:00')
	`, productID, name); err != nil {
		t.Fatalf("insert product: %v", err)
	}
}

func seedOrderExportOrder(t *testing.T, appDB *sqlx.Engine, orderID string, userID string, status string) {
	t.Helper()
	seedOrderExportOrderWithCreatedAt(t, appDB, orderID, userID, "", status, "2026-01-01 00:00:00")
}

func seedOrderExportOrderWithProduct(t *testing.T, appDB *sqlx.Engine, orderID string, userID string, productID string, status string) {
	t.Helper()
	seedOrderExportOrderWithCreatedAt(t, appDB, orderID, userID, productID, status, "2026-01-01 00:00:00")
}

func seedOrderExportOrderWithCreatedAt(t *testing.T, appDB *sqlx.Engine, orderID string, userID string, productID string, status string, createdAt string) {
	t.Helper()

	if _, err := appDB.ExecP(`
		INSERT INTO orders (id, user_id, product_id, amount, status, subscription_status, created_at)
		VALUES (?, ?, ?, 1000, ?, '', ?)
	`, orderID, userID, productID, status, createdAt); err != nil {
		t.Fatalf("insert order: %v", err)
	}
}
