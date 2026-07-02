package routes

import (
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

// --- Request DTOs ---

type StartConversationRequest struct {
	VisitorID      string `json:"visitor_id"`
	SourcePage     string `json:"source_page"`
	SourceReferrer string `json:"source_referrer"`
}

type SendMessageRequest struct {
	Message string `json:"message"`
}

// --- Response DTOs ---

type ConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	VisitorID      string `json:"visitor_id"`
}

type MessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type SendMessageResponse struct {
	Message           MessageResponse   `json:"message"`
	Citations         []CitationResponse `json:"citations,omitempty"`
	LeadCapturePrompt bool               `json:"lead_capture_prompt"`
}

type CitationResponse struct {
	ChunkID    string `json:"chunk_id"`
	SourceName string `json:"source_name"`
	Excerpt    string `json:"excerpt"`
}

// --- Converter helpers ---

func toMessageResponses(messages []usecase.MessageCo) []MessageResponse {
	result := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		result = append(result, MessageResponse{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}
	return result
}

func toCitationResponses(citations []usecase.ChatCitation) []CitationResponse {
	result := make([]CitationResponse, 0, len(citations))
	for _, c := range citations {
		result = append(result, CitationResponse{
			ChunkID:    c.ChunkID,
			SourceName: c.SourceName,
			Excerpt:    c.Excerpt,
		})
	}
	return result
}

// --- Rate limit constants ---

const (
	maxConversationsPerIPPerHour  = 20
	maxMessagesPerConversationPerHour = 30
	rateLimitWindow                = 1 * time.Hour
)

// --- Handlers ---

// StartSupportConversation creates or resumes a support conversation for an anonymous visitor.
// POST /api/public/support/conversations
func StartSupportConversation(c echo.Context) error {
	var req StartConversationRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	visitorIP := c.RealIP()
	if visitorIP == "" {
		visitorIP = "unknown"
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	// Rate limit: max 20 new conversations per IP per hour
	limited, err := usecase.CheckChatRateLimit(ctx, visitorIP, "ip_new_conversations", maxConversationsPerIPPerHour, rateLimitWindow)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	if limited {
		return httpresponse.Error(c, 429, "too many conversations, please try again later")
	}

	result, err := usecase.StartSupportConversation(ctx, usecase.StartConversationCmd{
		VisitorID:      strings.TrimSpace(req.VisitorID),
		VisitorIP:      visitorIP,
		SourcePage:     strings.TrimSpace(req.SourcePage),
		SourceReferrer: strings.TrimSpace(req.SourceReferrer),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ConversationResponse{
		ConversationID: result.ConversationID,
		VisitorID:      result.VisitorID,
	})
}

// SendSupportMessage sends a visitor message to a support conversation and gets an AI response.
// POST /api/public/support/conversations/:id/messages
func SendSupportMessage(c echo.Context) error {
	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		return httpresponse.BadRequest(c, "conversation ID is required")
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		return httpresponse.BadRequest(c, "message is required")
	}

	visitorIP := c.RealIP()
	if visitorIP == "" {
		visitorIP = "unknown"
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	// Verify the conversation exists and is active
	conversation, err := usecase.GetSupportConversationForChat(ctx, conversationID)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	// Verify conversation is open
	if conversation.Status != "open" && conversation.Status != "lead_captured" {
		return httpresponse.BadRequest(c, "this conversation is no longer active")
	}

	// Rate limit: max 30 messages per visitor per hour (using hashed key from conversation)
	limited, err := usecase.CheckChatRateLimitHashed(ctx, conversation.VisitorTokenHash, "visitor_messages", maxMessagesPerConversationPerHour, rateLimitWindow)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	if limited {
		return httpresponse.Error(c, 429, "too many messages, please try again later")
	}

	// Generate support answer via RAG pipeline
	response, err := usecase.GenerateSupportAnswer(ctx, usecase.SupportChatMessageCmd{
		ConversationID: conversationID,
		VisitorID:      "",
		Message:        message,
		SourcePage:     conversation.SourcePage,
		Referrer:       conversation.Referrer,
		VisitorToken:   "",
		VisitorIPHash:  visitorIP,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, SendMessageResponse{
		Message: MessageResponse{
			ID:        "",
			Role:      "assistant",
			Content:   response.Message,
			CreatedAt: "",
		},
		Citations:         toCitationResponses(response.Citations),
		LeadCapturePrompt: response.LeadCapturePrompt,
	})
}

// GetSupportMessages returns all messages for a conversation.
// GET /api/public/support/conversations/:id/messages
func GetSupportMessages(c echo.Context) error {
	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		return httpresponse.BadRequest(c, "conversation ID is required")
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	messages, err := usecase.ListSupportMessages(ctx, usecase.ListMessagesCmd{
		ConversationID: conversationID,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, toMessageResponses(messages))
}

// --- Lead capture ---

// SubmitLeadRequest DTO for lead capture submission.
type SubmitLeadRequest struct {
	Name            string `json:"name"`
	Company         string `json:"company"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	NeedDescription string `json:"need_description"`
}

// LeadResponse DTO returned after lead creation.
type LeadResponse struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Name           string `json:"name"`
	Company        string `json:"company"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	NeedDescription string `json:"need_description"`
	CreatedAt      string `json:"created_at"`
}

// SubmitSupportLead captures visitor contact info as a lead.
// POST /api/public/support/conversations/:id/lead
func SubmitSupportLead(c echo.Context) error {
	var req SubmitLeadRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		return httpresponse.BadRequest(c, "conversation ID is required")
	}

	ctx := fwcontext.InternalUsecaseContext(c)

	lead, err := usecase.CreateLead(ctx, usecase.CreateLeadCmd{
		ConversationID:  conversationID,
		Name:            req.Name,
		Company:         req.Company,
		Phone:           req.Phone,
		Email:           req.Email,
		NeedDescription: req.NeedDescription,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, LeadResponse{
		ID:              lead.ID,
		ConversationID:  lead.ConversationID,
		Name:            lead.Name,
		Company:         lead.Company,
		Email:           lead.ContactEmail,
		Phone:           lead.ContactPhone,
		NeedDescription: lead.NeedDescription,
		CreatedAt:       lead.CreatedAt,
	})
}
