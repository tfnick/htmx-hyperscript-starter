package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type QueueMessageResponse struct {
	ID             string `json:"id"`
	Queue          string `json:"queue"`
	BodyPreview    string `json:"body_preview"`
	Created        string `json:"created"`
	Updated        string `json:"updated"`
	Timeout        string `json:"timeout"`
	Received       int    `json:"received"`
	Priority       int    `json:"priority"`
	State          string `json:"state"`
	AgeSec         int64  `json:"age_sec"`
	AvailableInSec int64  `json:"available_in_sec"`
	NextClaimAt    string `json:"next_claim_at"`
	MaxReceive     int    `json:"max_receive"`
}

type QueueSummaryResponse struct {
	Queues []QueueSummaryItemResponse `json:"queues"`
}

type QueueSummaryItemResponse struct {
	Queue                 string `json:"queue"`
	Total                 int    `json:"total"`
	Ready                 int    `json:"ready"`
	Delayed               int    `json:"delayed"`
	InFlight              int    `json:"in_flight"`
	Exhausted             int    `json:"exhausted"`
	Retrying              int    `json:"retrying"`
	MaxReceived           int    `json:"max_received"`
	OldestReadyAgeSec     int64  `json:"oldest_ready_age_sec"`
	OldestInFlightAgeSec  int64  `json:"oldest_in_flight_age_sec"`
	OldestExhaustedAgeSec int64  `json:"oldest_exhausted_age_sec"`
	NextClaimAt           string `json:"next_claim_at"`
	NextClaimInSec        int64  `json:"next_claim_in_sec"`
}

type QueueMessageCorrelationResponse struct {
	MessageID  string                          `json:"message_id"`
	References []QueueMessageReferenceResponse `json:"references"`
}

type QueueMessageReferenceResponse struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Status      string `json:"status"`
	RelatedType string `json:"related_type"`
	RelatedID   string `json:"related_id"`
}

func ListMessages(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	messages, err := usecase.ListMessages(ctx, usecase.ListMessagesQry{
		Queue: c.QueryParam("queue"),
		State: c.QueryParam("state"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toQueueMessageResponses(messages))
}

func GetQueueSummary(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	summary, err := usecase.GetQueueSummary(ctx, usecase.QueueSummaryQry{
		Queue: c.QueryParam("queue"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toQueueSummaryResponse(summary))
}

func GetQueueMessageCorrelation(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	correlation, err := usecase.GetQueueMessageCorrelation(ctx, usecase.QueueMessageCorrelationQry{
		MessageID: c.Param("id"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, toQueueMessageCorrelationResponse(correlation))
}

func RetryQueueMessageNow(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.RetryQueueMessageNow(ctx, usecase.QueueMessageActionCmd{
		MessageID: c.Param("id"),
		Queue:     c.QueryParam("queue"),
	}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKMessage(c, "queue message retried")
}

func DeleteQueueMessage(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.DeleteQueueMessage(ctx, usecase.QueueMessageActionCmd{
		MessageID: c.Param("id"),
		Queue:     c.QueryParam("queue"),
	}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKMessage(c, "queue message deleted")
}

func toQueueMessageResponse(message usecase.QueueMessageCo) QueueMessageResponse {
	return QueueMessageResponse{
		ID:             message.ID,
		Queue:          message.Queue,
		BodyPreview:    message.BodyPreview,
		Created:        message.Created,
		Updated:        message.Updated,
		Timeout:        message.Timeout,
		Received:       message.Received,
		Priority:       message.Priority,
		State:          message.State,
		AgeSec:         message.AgeSec,
		AvailableInSec: message.AvailableInSec,
		NextClaimAt:    message.NextClaimAt,
		MaxReceive:     message.MaxReceive,
	}
}

func toQueueMessageResponses(messages []usecase.QueueMessageCo) []QueueMessageResponse {
	responses := make([]QueueMessageResponse, 0, len(messages))
	for i := range messages {
		responses = append(responses, toQueueMessageResponse(messages[i]))
	}
	return responses
}

func toQueueSummaryResponse(summary usecase.QueueSummaryCo) QueueSummaryResponse {
	response := QueueSummaryResponse{Queues: make([]QueueSummaryItemResponse, 0, len(summary.Queues))}
	for i := range summary.Queues {
		response.Queues = append(response.Queues, QueueSummaryItemResponse{
			Queue:                 summary.Queues[i].Queue,
			Total:                 summary.Queues[i].Total,
			Ready:                 summary.Queues[i].Ready,
			Delayed:               summary.Queues[i].Delayed,
			InFlight:              summary.Queues[i].InFlight,
			Exhausted:             summary.Queues[i].Exhausted,
			Retrying:              summary.Queues[i].Retrying,
			MaxReceived:           summary.Queues[i].MaxReceived,
			OldestReadyAgeSec:     summary.Queues[i].OldestReadyAgeSec,
			OldestInFlightAgeSec:  summary.Queues[i].OldestInFlightAgeSec,
			OldestExhaustedAgeSec: summary.Queues[i].OldestExhaustedAgeSec,
			NextClaimAt:           summary.Queues[i].NextClaimAt,
			NextClaimInSec:        summary.Queues[i].NextClaimInSec,
		})
	}
	return response
}

func toQueueMessageCorrelationResponse(correlation usecase.QueueMessageCorrelationCo) QueueMessageCorrelationResponse {
	response := QueueMessageCorrelationResponse{
		MessageID:  correlation.MessageID,
		References: make([]QueueMessageReferenceResponse, 0, len(correlation.References)),
	}
	for i := range correlation.References {
		response.References = append(response.References, QueueMessageReferenceResponse{
			Type:        correlation.References[i].Type,
			ID:          correlation.References[i].ID,
			Status:      correlation.References[i].Status,
			RelatedType: correlation.References[i].RelatedType,
			RelatedID:   correlation.References[i].RelatedID,
		})
	}
	return response
}
