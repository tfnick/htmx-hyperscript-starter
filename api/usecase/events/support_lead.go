package events

import (
	"fmt"

	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	SupportLeadCreatedTopic      = "support.lead.created"
	SupportLeadCreatedSubscriber = "external_notification.send_on_support_lead_created"
)

type SupportLeadCreatedPayload struct {
	LeadID              string `json:"lead_id"`
	ConversationID      string `json:"conversation_id"`
	Name                string `json:"name"`
	Company             string `json:"company"`
	ContactEmail        string `json:"contact_email"`
	ContactPhone        string `json:"contact_phone"`
	NeedDescription     string `json:"need_description"`
	SourcePage          string `json:"source_page"`
	DetectedIntent      string `json:"detected_intent"`
	ConversationSummary string `json:"conversation_summary"`
	CreatedAt           string `json:"created_at"`
}

func NewSupportLeadCreatedEvent(lead models.SupportLead) (fwevents.Event, error) {
	if lead.ID == "" {
		return fwevents.Event{}, fmt.Errorf("support lead ID is required")
	}
	return fwevents.NewPayloadEvent(SupportLeadCreatedTopic, "support_lead", lead.ID, SupportLeadCreatedPayload{
		LeadID:              lead.ID,
		ConversationID:      lead.ConversationID,
		Name:                lead.Name,
		Company:             lead.Company,
		ContactEmail:        lead.ContactEmail,
		ContactPhone:        lead.ContactPhone,
		NeedDescription:     lead.NeedDescription,
		SourcePage:          lead.SourcePage,
		DetectedIntent:      lead.DetectedIntent,
		ConversationSummary: lead.ConversationSummary,
		CreatedAt:           lead.CreatedAt,
	})
}
