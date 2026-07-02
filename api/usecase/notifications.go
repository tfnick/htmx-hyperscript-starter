package usecase

import (
	"github.com/google/uuid"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

type TriggerExportToastCmd struct {
	UserID string
}

func TriggerExportToast(ctx fwusecase.Context, cmd TriggerExportToastCmd) error {
	if cmd.UserID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}

	payload := map[string]string{
		"task_id": uuid.Must(uuid.NewV7()).String(),
		"status":  "completed",
		"message": "Export completed",
	}
	_, err := SendNotification(ctx, SendNotificationCmd{
		StorePolicy: StorePolicyStore,
		MessageType: RealtimeMessageTypeNotification,
		SourceType:  "experiment",
		SourceID:    "export-toast",
		UserID:      cmd.UserID,
		Title:       "Export completed",
		Summary:     "Export completed",
		Payload:     payload,
	})
	if err != nil {
		return fwusecase.E(fwusecase.CodeInternal, "failed to publish export notification", err)
	}
	return nil
}
