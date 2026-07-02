package usecase_test

import (
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
)

func sendRealtimeNotificationForUsecaseTest(ctx fwusecase.Context, cmd usecaseevents.SendRealtimeNotificationCmd) error {
	_, err := usecase.SendNotification(ctx, usecase.SendNotificationCmd{
		StorePolicy:  usecase.StorePolicyTransient,
		MessageType:  cmd.MessageType,
		Presentation: cmd.Presentation,
		UserID:       cmd.UserID,
		Payload:      cmd.Payload,
	})
	return err
}
