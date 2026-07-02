package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/framework/cache"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
	usecaseevents "github.com/tfnick/go-svelte-starter/api/usecase/events"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/embedding"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/kb"
	"github.com/tfnick/go-svelte-starter/api/usecase/integrations/llm"
)

const (
	// RAG constants
	defaultTopK         = 5
	defaultMinScore     = 0.5
	defaultMaxCitations = 5

	// Fallback message when no relevant context is found
	noContextFallback = "I don't have enough information to answer that question. Please try rephrasing or ask about our products and services."

	// System prompt for RAG-based answer generation
	ragSystemPrompt = `You are a helpful product consultant assistant. Answer the user's question using ONLY the provided knowledge base context below.

Rules:
1. Only use information from the provided context to answer.
2. If the context does not contain relevant information to answer the question, politely say: "I don't have enough information to answer that question. Please try rephrasing or ask about our products and services."
3. Do not make up information or use outside knowledge.
4. Be concise and helpful.
5. If the context contains pricing, features, or technical details, present them accurately.
6. If the question is a greeting or simple social message, you may respond naturally without requiring context.`
)

// --- Command and Response types ---

// SupportChatMessageCmd represents a visitor message in a support chat.
type SupportChatMessageCmd struct {
	ConversationID string
	VisitorID      string
	Message        string
	SourcePage     string
	Referrer       string
	VisitorToken   string
	VisitorIPHash  string
}

// SupportChatResponseCo represents the assistant's response.
type SupportChatResponseCo struct {
	Message           string
	Citations         []ChatCitation
	LeadCapturePrompt bool `json:"lead_capture_prompt"`
}

// ChatCitation represents a citation linking an answer to a KB chunk.
type ChatCitation struct {
	ChunkID    string
	SourceName string
	Excerpt    string
}

// GenerateSupportAnswer processes a visitor message through the RAG pipeline:
// 1. Embed the visitor question
// 2. Search for relevant chunks via vector similarity
// 3. Build a prompt with retrieved context
// 4. Generate an answer using the LLM
// 5. Return the answer with citations
func GenerateSupportAnswer(ctx fwusecase.Context, cmd SupportChatMessageCmd) (SupportChatResponseCo, error) {
	message := strings.TrimSpace(cmd.Message)
	if message == "" {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeValidation, "message is required", nil)
	}

	conversationID := strings.TrimSpace(cmd.ConversationID)
	if conversationID == "" {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeValidation, "conversation ID is required", nil)
	}

	// 1. Load embedding config
	embedCfg, err := cache.Cached(ctx.Std(), "config.embedding", "enabled:embed", 5*time.Minute, func() (models.IntegrationEmbeddingConfig, error) {
		return models.GetEnabledEmbeddingConfig(ctx.Std(), models.EmbeddingConfigQuery{
			Scenario:  models.IntegrationScenarioEmbedding,
			Operation: embeddingOperationCreate,
		})
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "embedding channel is not configured", err)
		}
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load embedding configuration", err)
	}

	// 2. Resolve embedding runtime. If the configured external provider has not
	// been fully configured yet, chat can fall back to the local hash provider so
	// existing local KB indexes remain usable.
	embedRuntime, err := supportChatEmbeddingRuntime(ctx.Std(), embedCfg)
	if err != nil {
		if errors.Is(err, errEmbeddingAdapterNotRegistered) {
			adapterKey := strings.TrimSpace(embedCfg.Channel.AdapterKey)
			if adapterKey == "" {
				adapterKey = "unknown"
			}
			return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal,
				fmt.Sprintf("embedding adapter not registered: %s", adapterKey), err)
		}
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal,
			"embedding provider config is invalid", err)
	}

	// 3. Generate embedding for the question
	embedResult, err := embedRuntime.Adapter.Embed(ctx.Std(), embedRuntime.ProviderConfig, embedding.EmbedRequest{
		Operation: embeddingOperationCreate,
		Texts:     []string{message},
		Params:    embedRuntime.ProviderConfig.ModelSettings,
	})
	if err != nil {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to generate question embedding", err)
	}

	if len(embedResult.Vectors) == 0 {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "embedding returned no vectors", nil)
	}

	// 5. Search for relevant chunks
	retriever := NewSQLiteVecRetriever()
	chunks, err := retriever.SearchHybrid(ctx.Std(), message, embedResult.Vectors[0].Values, defaultTopK, defaultMinScore)
	if err != nil {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to search knowledge base", err)
	}

	// 6. Store the visitor message
	_, err = models.AddSupportMessage(ctx.Std(), conversationID, models.SupportMessageRoleVisitor, message, "")
	if err != nil {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to store visitor message", err)
	}

	// 7. If no relevant chunks found, return fallback (lead capture may still apply)
	if len(chunks) == 0 {
		assistantMsg, err := models.AddSupportMessage(ctx.Std(), conversationID, models.SupportMessageRoleAssistant, noContextFallback, "no_results")
		if err != nil {
			return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to store assistant message", err)
		}
		_ = assistantMsg

		leadCapture := detectLeadCaptureIntent(message, conversationID, ctx)
		if leadCapture.triggered {
			_ = models.UpdateSupportConversationIntent(ctx.Std(), conversationID,
				models.SupportLeadCaptureRequested, leadCapture.intent)
		}

		return SupportChatResponseCo{
			Message:           noContextFallback,
			Citations:         nil,
			LeadCapturePrompt: leadCapture.triggered,
		}, nil
	}

	// 8. Load LLM config
	llmCfg, err := cache.Cached(ctx.Std(), "config.llm", "enabled:chat_completion", 5*time.Minute, func() (models.IntegrationLLMConfig, error) {
		return models.GetEnabledLLMConfig(ctx.Std(), models.LLMConfigQuery{
			Scenario:  models.IntegrationScenarioLLM,
			Operation: llm.OperationTextSummary,
		})
	})
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "LLM channel is not configured", err)
		}
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load LLM configuration", err)
	}

	// 9. Get LLM adapter
	llmAdapter, ok := registeredLLMAdapter(llmCfg.Channel.AdapterKey)
	if !ok {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal,
			fmt.Sprintf("LLM adapter not registered: %s", llmCfg.Channel.AdapterKey), nil)
	}

	// 10. Build LLM provider config
	llmProviderCfg, err := llmProviderConfig(llmCfg)
	if err != nil {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "LLM provider config is invalid", err)
	}

	// 11. Build prompt with retrieved context
	systemPrompt, userPrompt := buildRAGPrompt(chunks, message)

	// 12. Generate answer
	genResult, err := llmAdapter.Generate(ctx.Std(), llmProviderCfg, llm.GenerateRequest{
		Operation: llm.OperationTextSummary,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		// Store error message
		errorMsg := "Sorry, I encountered an error while processing your question. Please try again."
		_, _ = models.AddSupportMessage(ctx.Std(), conversationID, models.SupportMessageRoleAssistant, errorMsg, "generation_error")
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to generate answer", err)
	}

	answer := strings.TrimSpace(genResult.Content)
	if answer == "" {
		answer = noContextFallback
	}

	// 13. Build citations from retrieved chunks
	citationModels := make([]models.KnowledgeSearchResult, 0, len(chunks))
	citations := make([]ChatCitation, 0, len(chunks))
	for i, chunk := range chunks {
		if i >= defaultMaxCitations {
			break
		}
		sourceName := strings.TrimSpace(chunk.SourceName)
		if sourceName == "" {
			sourceName = "Unknown Source"
		}

		// Truncate excerpt to a reasonable length
		excerpt := chunk.Content
		if len([]rune(excerpt)) > 200 {
			excerpt = string([]rune(excerpt)[:200]) + "..."
		}

		citations = append(citations, ChatCitation{
			ChunkID:    chunk.ChunkID,
			SourceName: sourceName,
			Excerpt:    excerpt,
		})

		citationModels = append(citationModels, models.KnowledgeSearchResult{
			ChunkID:    chunk.ChunkID,
			SourceID:   chunk.SourceID,
			DocumentID: chunk.DocumentID,
			Content:    excerpt,
			Distance:   chunk.Score,
		})
	}

	// 14. Store the assistant message
	retrievalStatus := "retrieved"
	assistantMsg, err := models.AddSupportMessage(ctx.Std(), conversationID, models.SupportMessageRoleAssistant, answer, retrievalStatus)
	if err != nil {
		return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to store assistant message", err)
	}

	// 15. Store citations
	if len(citationModels) > 0 {
		if err := models.AddSupportCitations(ctx.Std(), assistantMsg.ID, conversationID, citationModels); err != nil {
			return SupportChatResponseCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to store citations", err)
		}
	}

	// 16. Detect lead capture intent
	leadCapture := detectLeadCaptureIntent(message, conversationID, ctx)

	// Update conversation with detected intent
	if leadCapture.triggered {
		if err := models.UpdateSupportConversationIntent(ctx.Std(), conversationID,
			models.SupportLeadCaptureRequested, leadCapture.intent); err != nil {
			// Non-fatal: intent update failure shouldn't block the response
		}
	}

	return SupportChatResponseCo{
		Message:           answer,
		Citations:         citations,
		LeadCapturePrompt: leadCapture.triggered,
	}, nil
}

// --- Conversation management types ---

// StartConversationCmd represents a request to start or resume a support conversation.
type StartConversationCmd struct {
	VisitorID      string
	VisitorIP      string
	SourcePage     string
	SourceReferrer string
}

// StartConversationCo represents the result of starting or resuming a conversation.
type StartConversationCo struct {
	ConversationID string
	VisitorID      string
}

// ListMessagesCmd represents a request to list messages for a conversation.
type ListMessagesCmd struct {
	ConversationID string
}

// MessageCo represents a message in a conversation.
type MessageCo struct {
	ID        string
	Role      string
	Content   string
	CreatedAt string
}

// ConversationCo represents a support conversation for route-level operations.
type ConversationCo struct {
	ID               string
	VisitorTokenHash string
	SourcePage       string
	Referrer         string
	Status           string
}

// GetSupportConversationForChat retrieves a conversation for the chat route handler.
func GetSupportConversationForChat(ctx fwusecase.Context, conversationID string) (ConversationCo, error) {
	id := strings.TrimSpace(conversationID)
	if id == "" {
		return ConversationCo{}, fwusecase.E(fwusecase.CodeValidation, "conversation ID is required", nil)
	}
	conv, err := models.GetSupportConversation(ctx.Std(), id)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ConversationCo{}, fwusecase.E(fwusecase.CodeNotFound, "conversation not found", err)
		}
		return ConversationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load conversation", err)
	}
	return ConversationCo{
		ID:               conv.ID,
		VisitorTokenHash: conv.VisitorTokenHash,
		SourcePage:       conv.SourcePage,
		Referrer:         conv.Referrer,
		Status:           conv.Status,
	}, nil
}

// StartSupportConversation creates or resumes a support conversation for an anonymous visitor.
// If visitorID is empty, a new UUID v7 is generated as the visitor token.
// The visitor token and IP are hashed with SHA-256 before storage for privacy.
func StartSupportConversation(ctx fwusecase.Context, cmd StartConversationCmd) (StartConversationCo, error) {
	visitorID := strings.TrimSpace(cmd.VisitorID)
	if visitorID == "" {
		visitorID = uuid.Must(uuid.NewV7()).String()
	}

	tokenHash := hashToken(visitorID)
	ipHash := hashToken(strings.TrimSpace(cmd.VisitorIP))

	sourcePage := strings.TrimSpace(cmd.SourcePage)
	referrer := strings.TrimSpace(cmd.SourceReferrer)

	conversation, err := models.GetOrCreateSupportConversation(ctx.Std(), tokenHash, ipHash, sourcePage, referrer)
	if err != nil {
		return StartConversationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create conversation", err)
	}

	return StartConversationCo{
		ConversationID: conversation.ID,
		VisitorID:      visitorID,
	}, nil
}

// ListSupportMessages returns all messages for a conversation, ordered by created_at.
func ListSupportMessages(ctx fwusecase.Context, cmd ListMessagesCmd) ([]MessageCo, error) {
	conversationID := strings.TrimSpace(cmd.ConversationID)
	if conversationID == "" {
		return nil, fwusecase.E(fwusecase.CodeValidation, "conversation ID is required", nil)
	}

	// Verify the conversation exists
	_, err := models.GetSupportConversation(ctx.Std(), conversationID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return nil, fwusecase.E(fwusecase.CodeNotFound, "conversation not found", err)
		}
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load conversation", err)
	}

	messages, err := models.ListSupportConversationMessages(ctx.Std(), conversationID)
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load messages", err)
	}

	result := make([]MessageCo, 0, len(messages))
	for _, m := range messages {
		result = append(result, MessageCo{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}
	return result, nil
}

// --- Rate limiting helpers ---

// CheckChatRateLimit checks whether the given key has exceeded the allowed count
// within a sliding time window. keyType is used for messages — "ip" or "visitor".
// The check uses a simple SQLite COUNT query.
func CheckChatRateLimit(ctx fwusecase.Context, key string, keyType string, maxCount int, window time.Duration) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return false, nil
	}

	hashedKey := hashToken(key)
	since := timefmt.SQLiteDateTime(time.Now().UTC().Add(-window))

	var count int
	var err error

	switch keyType {
	case "ip_new_conversations":
		count, err = models.CountRecentConversationsByIP(ctx.Std(), hashedKey, since)
	case "visitor_messages":
		count, err = models.CountRecentConversationMessagesByVisitor(ctx.Std(), hashedKey, since)
	default:
		return false, nil
	}

	if err != nil {
		return false, fwusecase.E(fwusecase.CodeInternal, "failed to check rate limit", err)
	}

	return count >= maxCount, nil
}

// CheckChatRateLimitHashed checks rate limit using an already-hashed key.
// This is used when the raw token is unavailable (e.g., from stored conversation lookup).
func CheckChatRateLimitHashed(ctx fwusecase.Context, hashedKey string, keyType string, maxCount int, window time.Duration) (bool, error) {
	key := strings.TrimSpace(hashedKey)
	if key == "" {
		return false, nil
	}

	since := timefmt.SQLiteDateTime(time.Now().UTC().Add(-window))

	var count int
	var err error

	switch keyType {
	case "visitor_messages":
		count, err = models.CountRecentConversationMessagesByVisitor(ctx.Std(), key, since)
	default:
		return false, nil
	}

	if err != nil {
		return false, fwusecase.E(fwusecase.CodeInternal, "failed to check rate limit", err)
	}

	return count >= maxCount, nil
}

// hashToken creates a SHA-256 hash of the given token for privacy-preserving storage.
func hashToken(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// buildRAGPrompt constructs the system and user prompts with retrieved context chunks.
func buildRAGPrompt(chunks []kb.Chunk, question string) (systemPrompt string, userPrompt string) {
	var contextBuilder strings.Builder
	for i, chunk := range chunks {
		if i > 0 {
			contextBuilder.WriteString("\n\n---\n\n")
		}
		contextBuilder.WriteString(fmt.Sprintf("[Source %d]\n%s", i+1, chunk.Content))
	}

	userPrompt = fmt.Sprintf("Context:\n\n%s\n\nQuestion: %s", contextBuilder.String(), question)
	return ragSystemPrompt, userPrompt
}

// --- Lead capture ---

// CreateLeadCmd represents a lead capture submission from a support chat visitor.
type CreateLeadCmd struct {
	ConversationID  string
	Name            string
	Company         string
	Phone           string
	Email           string
	NeedDescription string
}

// leadCaptureResult holds the result of lead capture intent detection.
type leadCaptureResult struct {
	triggered bool
	intent    string
}

// leadCaptureKeywords are keywords that indicate purchase/sales intent.
var leadCaptureKeywords = []string{
	"price", "pricing", "cost", "quote", "demo", "trial",
	"buy", "purchase", "contact", "follow up", "speak",
	"talk", "agent", "human", "representative",
}

// leadCapturePattern compiles the keywords into a case-insensitive regex.
var leadCapturePattern = regexp.MustCompile(
	`(?i)` + strings.Join(leadCaptureKeywords, "|"),
)

// detectLeadCaptureIntent checks whether lead capture should be triggered based on
// the visitor's message and conversation history (message count).
func detectLeadCaptureIntent(message string, conversationID string, ctx fwusecase.Context) leadCaptureResult {
	// Check keyword match in the current message
	if leadCapturePattern.MatchString(strings.ToLower(message)) {
		return leadCaptureResult{triggered: true, intent: "sales_intent_keyword"}
	}

	// Check if visitor has sent 3+ messages in this conversation
	conv, err := models.GetSupportConversation(ctx.Std(), conversationID)
	if err != nil {
		return leadCaptureResult{triggered: false}
	}
	if conv.MessageCount >= 3 {
		return leadCaptureResult{triggered: true, intent: "engaged_visitor"}
	}

	return leadCaptureResult{triggered: false}
}

// CreateLead validates and creates a support lead linked to an existing conversation.
func CreateLead(ctx fwusecase.Context, cmd CreateLeadCmd) (models.SupportLead, error) {
	conversationID := strings.TrimSpace(cmd.ConversationID)
	if conversationID == "" {
		return models.SupportLead{}, fwusecase.E(fwusecase.CodeValidation, "conversation ID is required", nil)
	}

	email := strings.TrimSpace(cmd.Email)
	phone := strings.TrimSpace(cmd.Phone)
	needDescription := strings.TrimSpace(cmd.NeedDescription)

	// Require at least phone OR email
	if email == "" && phone == "" {
		return models.SupportLead{}, fwusecase.E(fwusecase.CodeValidation, "either phone or email is required", nil)
	}
	if needDescription == "" {
		return models.SupportLead{}, fwusecase.E(fwusecase.CodeValidation, "need description is required", nil)
	}

	// Verify conversation exists
	conv, err := models.GetSupportConversation(ctx.Std(), conversationID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return models.SupportLead{}, fwusecase.E(fwusecase.CodeNotFound, "conversation not found", err)
		}
		return models.SupportLead{}, fwusecase.E(fwusecase.CodeInternal, "failed to load conversation", err)
	}

	// Build conversation summary from messages
	messages, err := models.ListSupportConversationMessages(ctx.Std(), conversationID)
	summary := ""
	if err == nil && len(messages) > 0 {
		var parts []string
		for _, m := range messages {
			if m.Role == models.SupportMessageRoleVisitor {
				parts = append(parts, m.Content)
			}
		}
		summary = strings.Join(parts, " | ")
		if len(summary) > 500 {
			summary = summary[:500]
		}
	}

	lead := models.SupportLead{
		ConversationID:      conversationID,
		Name:                strings.TrimSpace(cmd.Name),
		Company:             strings.TrimSpace(cmd.Company),
		ContactEmail:        email,
		ContactPhone:        phone,
		NeedDescription:     needDescription,
		SourcePage:          conv.SourcePage,
		DetectedIntent:      conv.DetectedIntent,
		ConversationSummary: summary,
	}

	var result models.SupportLead
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		created, err := models.CreateSupportLead(txCtx.Std(), lead)
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to create lead", err)
		}
		event, err := usecaseevents.NewSupportLeadCreatedEvent(created)
		if err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to build support lead event", err)
		}
		if err := fwevents.Publish(txCtx, event); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to publish support lead event", err)
		}
		result = created
		return nil
	})
	if err != nil {
		return models.SupportLead{}, err
	}
	return result, nil
}

// --- Support Console usecases ---

// ListConversationsCmd represents filters and pagination for listing support conversations.
type ListConversationsCmd struct {
	Page     int
	PageSize int
}

// ConversationSummaryCo represents a summary row in the admin conversation list.
type ConversationSummaryCo struct {
	ID               string
	VisitorID        string
	Status           string
	SourcePage       string
	CreatedAt        string
	MessageCount     int
	HasLead          bool
	LeadCaptureState string
	DetectedIntent   string
}

// ConversationDetailCo represents full conversation detail with messages and citations.
type ConversationDetailCo struct {
	Summary   ConversationSummaryCo
	Messages  []MessageCo
	Citations []CitationDetailCo
}

// CitationDetailCo represents a citation with resolved source info.
type CitationDetailCo struct {
	ID         string
	MessageID  string
	ChunkID    string
	SourceID   string
	SourceName string
	SourceType string
	Snippet    string
	Distance   float64
	CreatedAt  string
}

// LeadCo represents a lead in the admin console list.
type LeadCo struct {
	ID                  string
	ConversationID      string
	Name                string
	Company             string
	Email               string
	Phone               string
	NeedDescription     string
	Status              string
	ConversationSummary string
	DetectedIntent      string
	SourcePage          string
	CreatedAt           string
}

// LeadDetailCo represents a lead with its linked conversation info.
type LeadDetailCo struct {
	Lead         LeadCo
	Conversation ConversationSummaryCo
}

// ListSupportConversations returns a paginated list of support conversations for the admin console.
func ListSupportConversations(ctx fwusecase.Context, cmd ListConversationsCmd) ([]ConversationSummaryCo, int, error) {
	if cmd.Page < 1 {
		cmd.Page = 1
	}
	if cmd.PageSize < 1 {
		cmd.PageSize = 20
	}

	conversations, err := models.ListSupportConversations(ctx.Std())
	if err != nil {
		return nil, 0, fwusecase.E(fwusecase.CodeInternal, "failed to load conversations", err)
	}

	// Simple in-memory pagination (MVP with limited data volume)
	total := len(conversations)
	start := (cmd.Page - 1) * cmd.PageSize
	if start >= total {
		return []ConversationSummaryCo{}, total, nil
	}
	end := start + cmd.PageSize
	if end > total {
		end = total
	}
	page := conversations[start:end]

	var result []ConversationSummaryCo
	for _, conv := range page {
		hasLead := conv.Status == models.SupportConversationStatusLeadCaptured
		result = append(result, ConversationSummaryCo{
			ID:               conv.ID,
			VisitorID:        conv.VisitorTokenHash,
			Status:           conv.Status,
			SourcePage:       conv.SourcePage,
			CreatedAt:        conv.CreatedAt,
			MessageCount:     conv.MessageCount,
			HasLead:          hasLead,
			LeadCaptureState: conv.LeadCaptureState,
			DetectedIntent:   conv.DetectedIntent,
		})
	}
	return result, total, nil
}

// GetSupportConversationDetail returns the full detail for a support conversation.
func GetSupportConversationDetail(ctx fwusecase.Context, conversationID string) (ConversationDetailCo, error) {
	if strings.TrimSpace(conversationID) == "" {
		return ConversationDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "conversation ID is required", nil)
	}

	conv, err := models.GetSupportConversation(ctx.Std(), conversationID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ConversationDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "conversation not found", err)
		}
		return ConversationDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load conversation", err)
	}

	hasLead := conv.Status == models.SupportConversationStatusLeadCaptured
	summary := ConversationSummaryCo{
		ID:               conv.ID,
		VisitorID:        conv.VisitorTokenHash,
		Status:           conv.Status,
		SourcePage:       conv.SourcePage,
		CreatedAt:        conv.CreatedAt,
		MessageCount:     conv.MessageCount,
		HasLead:          hasLead,
		LeadCaptureState: conv.LeadCaptureState,
		DetectedIntent:   conv.DetectedIntent,
	}

	// Load messages
	msgs, err := models.ListSupportConversationMessages(ctx.Std(), conversationID)
	if err != nil {
		return ConversationDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load messages", err)
	}
	messages := make([]MessageCo, 0, len(msgs))
	for _, m := range msgs {
		messages = append(messages, MessageCo{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}

	// Load citations
	citationModels, err := models.ListSupportMessageCitations(ctx.Std(), conversationID)
	if err != nil {
		return ConversationDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load citations", err)
	}
	citations := make([]CitationDetailCo, 0, len(citationModels))
	for _, c := range citationModels {
		citations = append(citations, CitationDetailCo{
			ID:         c.ID,
			MessageID:  c.MessageID,
			ChunkID:    c.ChunkID,
			SourceID:   c.SourceID,
			SourceName: c.Title,
			SourceType: c.SourceType,
			Snippet:    c.Snippet,
			Distance:   c.Distance,
			CreatedAt:  c.CreatedAt,
		})
	}

	return ConversationDetailCo{
		Summary:   summary,
		Messages:  messages,
		Citations: citations,
	}, nil
}

// ListSupportLeads returns a paginated list of support leads for the admin console.
func ListSupportLeads(ctx fwusecase.Context, cmd ListConversationsCmd) ([]LeadCo, int, error) {
	if cmd.Page < 1 {
		cmd.Page = 1
	}
	if cmd.PageSize < 1 {
		cmd.PageSize = 20
	}

	leads, err := models.ListSupportLeads(ctx.Std())
	if err != nil {
		return nil, 0, fwusecase.E(fwusecase.CodeInternal, "failed to load leads", err)
	}

	total := len(leads)
	start := (cmd.Page - 1) * cmd.PageSize
	if start >= total {
		return []LeadCo{}, total, nil
	}
	end := start + cmd.PageSize
	if end > total {
		end = total
	}
	page := leads[start:end]

	var result []LeadCo
	for _, lead := range page {
		result = append(result, LeadCo{
			ID:                  lead.ID,
			ConversationID:      lead.ConversationID,
			Name:                lead.Name,
			Company:             lead.Company,
			Email:               lead.ContactEmail,
			Phone:               lead.ContactPhone,
			NeedDescription:     lead.NeedDescription,
			ConversationSummary: lead.ConversationSummary,
			DetectedIntent:      lead.DetectedIntent,
			SourcePage:          lead.SourcePage,
			CreatedAt:           lead.CreatedAt,
		})
	}
	return result, total, nil
}

// GetSupportLeadDetail returns a lead with its linked conversation summary.
func GetSupportLeadDetail(ctx fwusecase.Context, leadID string) (LeadDetailCo, error) {
	if strings.TrimSpace(leadID) == "" {
		return LeadDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "lead ID is required", nil)
	}

	lead, err := models.GetSupportLead(ctx.Std(), leadID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return LeadDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "lead not found", err)
		}
		return LeadDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load lead", err)
	}

	leadCo := LeadCo{
		ID:                  lead.ID,
		ConversationID:      lead.ConversationID,
		Name:                lead.Name,
		Company:             lead.Company,
		Email:               lead.ContactEmail,
		Phone:               lead.ContactPhone,
		NeedDescription:     lead.NeedDescription,
		ConversationSummary: lead.ConversationSummary,
		DetectedIntent:      lead.DetectedIntent,
		SourcePage:          lead.SourcePage,
		CreatedAt:           lead.CreatedAt,
	}

	// Load linked conversation summary
	var convSummary ConversationSummaryCo
	if lead.ConversationID != "" {
		conv, err := models.GetSupportConversation(ctx.Std(), lead.ConversationID)
		if err == nil {
			hasLead := conv.Status == models.SupportConversationStatusLeadCaptured
			convSummary = ConversationSummaryCo{
				ID:               conv.ID,
				VisitorID:        conv.VisitorTokenHash,
				Status:           conv.Status,
				SourcePage:       conv.SourcePage,
				CreatedAt:        conv.CreatedAt,
				MessageCount:     conv.MessageCount,
				HasLead:          hasLead,
				LeadCaptureState: conv.LeadCaptureState,
				DetectedIntent:   conv.DetectedIntent,
			}
		}
	}

	return LeadDetailCo{
		Lead:         leadCo,
		Conversation: convSummary,
	}, nil
}

// LeadStatusCo represents a lead with its status in the admin console.
type LeadStatusCo struct {
	LeadCo
	Status string
}
