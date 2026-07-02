package realtime

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypePoints          MessageType = "points"
	MessageTypeAsyncExportTask MessageType = "async_export_task"
	MessageTypeNotification    MessageType = "notification"
	MessageTypeHeavyTask       MessageType = "heavy_task"
)

type Presentation string

const (
	PresentationRefresh Presentation = "refresh"
	PresentationToast   Presentation = "toast"
)

type Message struct {
	Type         MessageType  `json:"type"`
	Presentation Presentation `json:"presentation"`
	Payload      interface{}  `json:"payload,omitempty"`
}

type PointsPayload struct {
	UserID   string `json:"user_id"`
	ClientID string `json:"client_id,omitempty"`
	Balance  int64  `json:"balance"`
}

type AsyncExportTaskPayload struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type NotificationPayload struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Summary    string `json:"summary,omitempty"`
	SourceType string `json:"source_type,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

type Subscription struct {
	Messages <-chan []byte

	hub      *Hub
	userID   string
	clientID string
	ch       chan []byte
	once     sync.Once
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*Subscription]struct{}
	clients     map[string]*Subscription
}

var defaultHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[*Subscription]struct{}),
		clients:     make(map[string]*Subscription),
	}
}

func Subscribe(userID string) *Subscription {
	return defaultHub.Subscribe(userID)
}

func SubscribeClient(userID string, clientID string) *Subscription {
	return defaultHub.SubscribeClient(userID, clientID)
}

func Publish(userID string, message interface{}) error {
	return defaultHub.Publish(userID, message)
}

func PublishClient(clientID string, message interface{}) error {
	return defaultHub.PublishClient(clientID, message)
}

func NewMessage(messageType MessageType, presentation Presentation, payload interface{}) Message {
	return NormalizeMessage(Message{
		Type:         messageType,
		Presentation: presentation,
		Payload:      payload,
	})
}

func NewPointsMessage(payload PointsPayload, presentation Presentation) Message {
	return NewMessage(MessageTypePoints, presentation, payload)
}

func NewAsyncExportTaskMessage(payload AsyncExportTaskPayload, presentation Presentation) Message {
	return NewMessage(MessageTypeAsyncExportTask, presentation, payload)
}

func NewNotificationMessage(payload NotificationPayload, presentation Presentation) Message {
	return NewMessage(MessageTypeNotification, presentation, payload)
}

func DefaultPresentation(messageType MessageType) Presentation {
	switch messageType {
	case MessageTypeAsyncExportTask, MessageTypeNotification, MessageTypeHeavyTask:
		return PresentationToast
	case MessageTypePoints:
		return PresentationRefresh
	default:
		return PresentationRefresh
	}
}

func NormalizeMessage(message Message) Message {
	if message.Presentation == "" {
		message.Presentation = DefaultPresentation(message.Type)
	}
	return message
}

func (h *Hub) Subscribe(userID string) *Subscription {
	return h.SubscribeClient(userID, "")
}

func (h *Hub) SubscribeClient(userID string, clientID string) *Subscription {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = uuid.Must(uuid.NewV7()).String()
	}

	ch := make(chan []byte, 16)
	sub := &Subscription{
		Messages: ch,
		hub:      h,
		userID:   userID,
		clientID: clientID,
		ch:       ch,
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	byUser := h.subscribers[userID]
	if byUser == nil {
		byUser = make(map[*Subscription]struct{})
		h.subscribers[userID] = byUser
	}
	byUser[sub] = struct{}{}
	h.clients[clientID] = sub
	return sub
}

func (s *Subscription) UserID() string {
	if s == nil {
		return ""
	}
	return s.userID
}

func (s *Subscription) ClientID() string {
	if s == nil {
		return ""
	}
	return s.clientID
}

func (s *Subscription) Close() {
	if s == nil || s.hub == nil {
		return
	}

	s.once.Do(func() {
		s.hub.unsubscribe(s)
	})
}

func (h *Hub) Publish(userID string, message interface{}) error {
	payload, err := marshalMessage(message)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for sub := range h.subscribers[userID] {
		select {
		case sub.ch <- payload:
		default:
		}
	}
	return nil
}

func (h *Hub) PublishClient(clientID string, message interface{}) error {
	payload, err := marshalMessage(message)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	sub := h.clients[clientID]
	if sub == nil {
		return nil
	}
	select {
	case sub.ch <- payload:
	default:
	}
	return nil
}

func marshalMessage(message interface{}) ([]byte, error) {
	switch typed := message.(type) {
	case Message:
		return json.Marshal(NormalizeMessage(typed))
	case *Message:
		if typed == nil {
			return json.Marshal(nil)
		}
		return json.Marshal(NormalizeMessage(*typed))
	default:
		return json.Marshal(message)
	}
}

func (h *Hub) unsubscribe(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()

	byUser := h.subscribers[sub.userID]
	if byUser != nil {
		delete(byUser, sub)
		if len(byUser) == 0 {
			delete(h.subscribers, sub.userID)
		}
	}
	if h.clients[sub.clientID] == sub {
		delete(h.clients, sub.clientID)
	}
	close(sub.ch)
}
