package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type SummarizeTextRequest struct {
	Text       string   `json:"text"`
	Prompt     string   `json:"prompt"`
	Dimensions []string `json:"dimensions"`
}

type LLMSummaryResponse struct {
	Summary      map[string]string `json:"summary"`
	ModelCode    string            `json:"model_code"`
	ChannelCode  string            `json:"channel_code"`
	InvocationID string            `json:"invocation_id"`
}

func ToLLMSummaryResponse(summary usecase.LLMSummaryCo) LLMSummaryResponse {
	return LLMSummaryResponse{
		Summary:      summary.Summary,
		ModelCode:    summary.ModelCode,
		ChannelCode:  summary.ChannelCode,
		InvocationID: summary.InvocationID,
	}
}

func SummarizeTextWithLLM(c echo.Context) error {
	var req SummarizeTextRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	summary, err := usecase.SummarizeTextWithLLM(ctx, usecase.SummarizeTextWithLLMCmd{
		Text:       req.Text,
		Prompt:     req.Prompt,
		Dimensions: req.Dimensions,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToLLMSummaryResponse(summary))
}
