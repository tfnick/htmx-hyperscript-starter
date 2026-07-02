package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// --- Bot list ---

// BotListItem represents a single enabled bot channel for the experiment tab.
type BotListItem struct {
	ChannelCode  string `json:"channel_code"`
	ProviderCode string `json:"provider_code"`
	AdapterKey   string `json:"adapter_key"`
}

// ListBotsForExperiment returns enabled external notification channel configs.
// GET /api/admin/experiments/bots
func ListBotsForExperiment(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)

	bots, err := usecase.ListEnabledBots(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	var items []BotListItem
	for _, b := range bots {
		items = append(items, BotListItem{
			ChannelCode:  b.ChannelCode,
			ProviderCode: b.ProviderCode,
			AdapterKey:   b.AdapterKey,
		})
	}

	return httpresponse.OK(c, items)
}

// --- Bot send ---

// SendExperimentBotMessageRequest is the request body for sending a test bot message.
type SendExperimentBotMessageRequest struct {
	ChannelCode string `json:"channel_code"`
	Title       string `json:"title"`
	Content     string `json:"content"`
}

// SendExperimentBotMessageResponse is the response for a successful bot test send.
type SendExperimentBotMessageResponse struct {
	Success          bool   `json:"success"`
	ResponseSnapshot string `json:"response_snapshot"`
}

// SendExperimentBotMessage sends a test message via the selected bot channel.
// POST /api/admin/experiments/bots/send
func SendExperimentBotMessage(c echo.Context) error {
	var req SendExperimentBotMessageRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.SendTestBotMessage(ctx, usecase.SendTestBotMessageCmd{
		ChannelCode: req.ChannelCode,
		Title:       req.Title,
		Content:     req.Content,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, SendExperimentBotMessageResponse{
		Success:          true,
		ResponseSnapshot: result.ResponseSnapshot,
	})
}
