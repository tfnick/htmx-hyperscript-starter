package usecase_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/db"
	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
)

func TestSupportLeadCreatedExternalNotificationSkipsWhenNoBotConfigured(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	registerExternalNotificationEventHandlerForTest(t)
	seedSupportConversationForLead(t, "conv-1")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	lead, err := usecase.CreateLead(ctx, usecase.CreateLeadCmd{
		ConversationID:  "conv-1",
		Name:            "Ada",
		Email:           "ada@example.com",
		NeedDescription: "Need pricing",
	})
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}

	if eventCount := countRows(t, appDB, `SELECT COUNT(*) FROM domain_events WHERE topic = ? AND aggregate_id = ?`, usecaseevents.SupportLeadCreatedTopic, lead.ID); eventCount != 1 {
		t.Fatalf("expected support lead event, got %d", eventCount)
	}
	handleDomainEventMessageForSubscriber(t, appDB, usecaseevents.SupportLeadCreatedSubscriber)

	var delivery models.ExternalNotificationDelivery
	if err := appDB.Get(&delivery, externalNotificationDeliveryTestSelect()+` LIMIT 1`); err != nil {
		t.Fatalf("load skipped delivery: %v", err)
	}
	if delivery.Status != models.ExternalNotificationDeliveryStatusSkipped {
		t.Fatalf("expected skipped delivery, got %q", delivery.Status)
	}
	if delivery.Note != "未配置bot" {
		t.Fatalf("expected no bot note, got %q", delivery.Note)
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueExternalNotifications); queueCount != 0 {
		t.Fatalf("expected no external notification queue message, got %d", queueCount)
	}
}

func TestSupportLeadCreatedExternalNotificationQueuesWhenBotConfiguredWithoutPrimary(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)
	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	registerExternalNotificationEventHandlerForTest(t)
	seedSupportConversationForLead(t, "conv-2")

	ctx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	channel := createExternalNotificationBotForTest(t, ctx, false)

	lead, err := usecase.CreateLead(ctx, usecase.CreateLeadCmd{
		ConversationID:  "conv-2",
		Name:            "Grace",
		Email:           "grace@example.com",
		NeedDescription: "Need enterprise plan",
	})
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	handleDomainEventMessageForSubscriber(t, appDB, usecaseevents.SupportLeadCreatedSubscriber)

	var delivery models.ExternalNotificationDelivery
	if err := appDB.Get(&delivery, externalNotificationDeliveryTestSelect()+` WHERE event_id IN (
		SELECT id FROM domain_events WHERE topic = ? AND aggregate_id = ?
	) LIMIT 1`, usecaseevents.SupportLeadCreatedTopic, lead.ID); err != nil {
		t.Fatalf("load queued delivery: %v", err)
	}
	if delivery.Status != models.ExternalNotificationDeliveryStatusQueued {
		t.Fatalf("expected queued delivery, got %q", delivery.Status)
	}
	if delivery.ChannelID != channel.ID {
		t.Fatalf("expected delivery channel %q, got %q", channel.ID, delivery.ChannelID)
	}
	if delivery.Note == "未配置bot" {
		t.Fatalf("configured bot must not be treated as missing")
	}
	if queueCount := countRows(t, appDB, `SELECT COUNT(*) FROM goqite WHERE queue = ?`, queue.QueueExternalNotifications); queueCount != 1 {
		t.Fatalf("expected one external notification queue message, got %d", queueCount)
	}
}

func registerExternalNotificationEventHandlerForTest(t *testing.T) {
	t.Helper()

	queueManager, err := queue.NewManager()
	if err != nil {
		t.Fatalf("new queue manager: %v", err)
	}
	usecase.DefaultQueueManager = queueManager
	t.Cleanup(func() {
		usecase.DefaultQueueManager = nil
	})
	fwevents.Configure(usecaseevents.DurableStore{}, queueManager)
	if err := usecase.RegisterExternalNotificationEventHandlers(); err != nil {
		t.Fatalf("register external notification handler: %v", err)
	}
}

func seedSupportConversationForLead(t *testing.T, conversationID string) {
	t.Helper()

	appDB, err := db.DefaultManager.GetEngine("app")
	if err != nil {
		t.Fatalf("get app engine: %v", err)
	}
	if _, err := appDB.ExecP(`
		INSERT INTO support_conversations (
			id, visitor_token_hash, visitor_ip_hash, source_page, referrer,
			status, lead_capture_state, message_count, detected_intent, created_at, updated_at
		) VALUES (?, 'visitor-hash', 'ip-hash', '/pricing', '', 'open', 'requested', 1, 'sales_intent_keyword', '2026-06-15 00:00:00', '2026-06-15 00:00:00')
	`, conversationID); err != nil {
		t.Fatalf("insert support conversation: %v", err)
	}
}

func createExternalNotificationBotForTest(t *testing.T, ctx fwusecase.Context, primary bool) models.IntegrationChannelConfig {
	t.Helper()

	credential, err := models.CreateIntegrationCredential(ctx.Std(), models.CreateIntegrationCredentialCmd{
		CredentialType: "webhook_url",
		ValueText:      `{"webhook_url":"https://example.test/feishu"}`,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("create integration credential: %v", err)
	}
	channel, err := models.CreateIntegrationChannel(ctx.Std(), models.CreateIntegrationChannelCmd{
		Scenario:     models.IntegrationScenarioExternalNotification,
		ChannelCode:  "feishu-sales",
		ProviderCode: "feishu",
		AdapterKey:   "bot.feishu.webhook",
		Environment:  "production",
		Enabled:      true,
		Priority:     100,
		CredentialID: credential.ID,
		IsPrimary:    primary,
		ConfigJSON:   `{"message_type":"text"}`,
		MetadataJSON: `{}`,
	})
	if err != nil {
		t.Fatalf("create integration channel: %v", err)
	}
	return channel
}

func externalNotificationDeliveryTestSelect() string {
	return `
		SELECT id, event_id, channel_id, provider_code, adapter_key, idempotency_key,
			status, attempts, COALESCE(next_attempt_at, '') AS next_attempt_at,
			last_error_code, last_error_message, request_snapshot_json, response_snapshot_json,
			note, COALESCE(sent_at, '') AS sent_at, message_id, created_at, updated_at
		FROM external_notification_deliveries
	`
}
