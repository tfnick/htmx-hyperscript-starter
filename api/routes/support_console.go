package routes

import (
	"errors"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/models"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// --- Response DTOs ---

// AdminConversationResponse DTO for conversation list items.
type AdminConversationResponse struct {
	ID               string `json:"id"`
	VisitorID        string `json:"visitor_id"`
	Status           string `json:"status"`
	SourcePage       string `json:"source_page"`
	CreatedAt        string `json:"created_at"`
	MessageCount     int    `json:"message_count"`
	HasLead          bool   `json:"has_lead"`
	LeadCaptureState string `json:"lead_capture_state"`
	DetectedIntent   string `json:"detected_intent,omitempty"`
}

// AdminMessageResponse DTO for messages in conversation detail.
type AdminMessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// AdminCitationResponse DTO for citations in conversation detail.
type AdminCitationResponse struct {
	ID         string  `json:"id"`
	MessageID  string  `json:"message_id"`
	ChunkID    string  `json:"chunk_id"`
	SourceID   string  `json:"source_id"`
	SourceName string  `json:"source_name"`
	SourceType string  `json:"source_type"`
	Snippet    string  `json:"snippet"`
	Distance   float64 `json:"distance"`
}

// AdminConversationDetailResponse DTO for conversation detail.
type AdminConversationDetailResponse struct {
	Conversation AdminConversationResponse  `json:"conversation"`
	Messages     []AdminMessageResponse     `json:"messages"`
	Citations    []AdminCitationResponse    `json:"citations,omitempty"`
}

// AdminConversationListResponse DTO for paginated conversation list.
type AdminConversationListResponse struct {
	Items      []AdminConversationResponse `json:"items"`
	Total      int                         `json:"total"`
	Page       int                         `json:"page"`
	PageSize   int                         `json:"page_size"`
}

// AdminLeadResponse DTO for lead list items.
type AdminLeadResponse struct {
	ID                  string `json:"id"`
	ConversationID      string `json:"conversation_id"`
	Name                string `json:"name"`
	Company             string `json:"company"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	NeedDescription     string `json:"need_description"`
	ConversationSummary string `json:"conversation_summary,omitempty"`
	DetectedIntent      string `json:"detected_intent,omitempty"`
	SourcePage          string `json:"source_page,omitempty"`
	CreatedAt           string `json:"created_at"`
}

// AdminLeadDetailResponse DTO for lead detail.
type AdminLeadDetailResponse struct {
	Lead         AdminLeadResponse          `json:"lead"`
	Conversation AdminConversationResponse  `json:"conversation,omitempty"`
}

// AdminLeadListResponse DTO for paginated lead list.
type AdminLeadListResponse struct {
	Items    []AdminLeadResponse `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// --- Converter helpers ---

func toAdminConversationResponse(c usecase.ConversationSummaryCo) AdminConversationResponse {
	return AdminConversationResponse{
		ID:               c.ID,
		VisitorID:        c.VisitorID,
		Status:           c.Status,
		SourcePage:       c.SourcePage,
		CreatedAt:        c.CreatedAt,
		MessageCount:     c.MessageCount,
		HasLead:          c.HasLead,
		LeadCaptureState: c.LeadCaptureState,
		DetectedIntent:   c.DetectedIntent,
	}
}

func toAdminMessageResponse(m usecase.MessageCo) AdminMessageResponse {
	return AdminMessageResponse{
		ID:        m.ID,
		Role:      m.Role,
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}
}

func toAdminCitationResponse(c usecase.CitationDetailCo) AdminCitationResponse {
	return AdminCitationResponse{
		ID:         c.ID,
		MessageID:  c.MessageID,
		ChunkID:    c.ChunkID,
		SourceID:   c.SourceID,
		SourceName: c.SourceName,
		SourceType: c.SourceType,
		Snippet:    c.Snippet,
		Distance:   c.Distance,
	}
}

func toAdminLeadResponse(l usecase.LeadCo) AdminLeadResponse {
	return AdminLeadResponse{
		ID:                  l.ID,
		ConversationID:      l.ConversationID,
		Name:                l.Name,
		Company:             l.Company,
		Email:               l.Email,
		Phone:               l.Phone,
		NeedDescription:     l.NeedDescription,
		ConversationSummary: l.ConversationSummary,
		DetectedIntent:      l.DetectedIntent,
		SourcePage:          l.SourcePage,
		CreatedAt:           l.CreatedAt,
	}
}

func parsePagination(c echo.Context) (int, int) {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

// --- Conversation handlers ---

// ListSupportConversationsAdmin lists all support conversations for the admin console.
// GET /api/admin/support/conversations
func ListSupportConversationsAdmin(c echo.Context) error {
	page, pageSize := parsePagination(c)
	ctx := fwcontext.InternalUsecaseContext(c)

	items, total, err := usecase.ListSupportConversations(ctx, usecase.ListConversationsCmd{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	var responses []AdminConversationResponse
	for _, item := range items {
		responses = append(responses, toAdminConversationResponse(item))
	}

	return httpresponse.OK(c, AdminConversationListResponse{
		Items:    responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetSupportConversationAdmin returns full detail for a support conversation.
// GET /api/admin/support/conversations/:id
func GetSupportConversationAdmin(c echo.Context) error {
	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		return httpresponse.BadRequest(c, "conversation ID is required")
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	detail, err := usecase.GetSupportConversationDetail(ctx, conversationID)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	var messages []AdminMessageResponse
	for _, m := range detail.Messages {
		messages = append(messages, toAdminMessageResponse(m))
	}

	var citations []AdminCitationResponse
	for _, c := range detail.Citations {
		citations = append(citations, toAdminCitationResponse(c))
	}

	return httpresponse.OK(c, AdminConversationDetailResponse{
		Conversation: toAdminConversationResponse(detail.Summary),
		Messages:     messages,
		Citations:    citations,
	})
}

// --- Lead handlers ---

// ListSupportLeadsAdmin lists all support leads for the admin console.
// GET /api/admin/support/leads
func ListSupportLeadsAdmin(c echo.Context) error {
	page, pageSize := parsePagination(c)
	ctx := fwcontext.InternalUsecaseContext(c)

	items, total, err := usecase.ListSupportLeads(ctx, usecase.ListConversationsCmd{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	var responses []AdminLeadResponse
	for _, item := range items {
		responses = append(responses, toAdminLeadResponse(item))
	}

	return httpresponse.OK(c, AdminLeadListResponse{
		Items:    responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetSupportLeadAdmin returns full detail for a support lead.
// GET /api/admin/support/leads/:id
func GetSupportLeadAdmin(c echo.Context) error {
	leadID := strings.TrimSpace(c.Param("id"))
	if leadID == "" {
		return httpresponse.BadRequest(c, "lead ID is required")
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	detail, err := usecase.GetSupportLeadDetail(ctx, leadID)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, AdminLeadDetailResponse{
		Lead:         toAdminLeadResponse(detail.Lead),
		Conversation: toAdminConversationResponse(detail.Conversation),
	})
}

// --- Bot config status ---

// BotConfigStatusResponse response for bot configuration status check.
type BotConfigStatusResponse struct {
	Configured bool   `json:"configured"`
	BotName    string `json:"bot_name,omitempty"`
}

// GetBotConfigStatus returns the current notification bot configuration status.
// GET /api/admin/support/bot-config-status
func GetBotConfigStatus(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)

	channel, err := models.GetEnabledPrimaryOrDefaultIntegrationChannelConfig(ctx.Std(), models.IntegrationScenarioExternalNotification)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return httpresponse.OK(c, BotConfigStatusResponse{
				Configured: false,
			})
		}
		return httpresponse.InternalUsecaseError(c, err)
	}

	botName := channel.ChannelCode
	if botName == "" {
		botName = channel.ProviderCode
	}

	return httpresponse.OK(c, BotConfigStatusResponse{
		Configured: true,
		BotName:    botName,
	})
}
