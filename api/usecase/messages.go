package usecase

import (
	"errors"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/queue"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type ListMessagesQry struct {
	Queue string
	State string
}

type QueueSummaryQry struct {
	Queue string
}

type QueueMessageCorrelationQry struct {
	MessageID string
}

type QueueMessageActionCmd struct {
	MessageID string
	Queue     string
}

type QueueMessageCo struct {
	ID             string
	Queue          string
	BodyPreview    string
	Created        string
	Updated        string
	Timeout        string
	Received       int
	Priority       int
	State          string
	AgeSec         int64
	AvailableInSec int64
	NextClaimAt    string
	MaxReceive     int
}

type QueueSummaryCo struct {
	Queues []QueueSummaryItemCo
}

type QueueSummaryItemCo struct {
	Queue                 string
	Total                 int
	Ready                 int
	Delayed               int
	InFlight              int
	Exhausted             int
	Retrying              int
	MaxReceived           int
	OldestReadyAgeSec     int64
	OldestInFlightAgeSec  int64
	OldestExhaustedAgeSec int64
	NextClaimAt           string
	NextClaimInSec        int64
}

type QueueMessageCorrelationCo struct {
	MessageID  string
	References []QueueMessageReferenceCo
}

type QueueMessageReferenceCo struct {
	Type        string
	ID          string
	Status      string
	RelatedType string
	RelatedID   string
}

func ListMessages(ctx fwusecase.Context, qry ListMessagesQry) ([]QueueMessageCo, error) {
	stateFilter, err := normalizeQueueMessageState(qry.State)
	if err != nil {
		return nil, err
	}
	messages, err := models.ListQueueMessages(ctx.Std(), strings.TrimSpace(qry.Queue))
	if err != nil {
		return nil, fwusecase.E(fwusecase.CodeInternal, "failed to load queue messages", err)
	}

	now := timefmt.NowUTC()
	result := make([]QueueMessageCo, 0, len(messages))
	for i := range messages {
		state := classifyQueueMessage(messages[i].Received, messages[i].Timeout, now)
		if stateFilter != "" && state != stateFilter {
			continue
		}
		createdAt, hasCreated := parseQueueTime(messages[i].Created)
		timeoutAt, hasTimeout := parseQueueTime(messages[i].Timeout)
		nextClaimAt := ""
		if (state == QueueMessageStateDelayed || state == QueueMessageStateInFlight) && hasTimeout {
			nextClaimAt = timefmt.RFC3339(timeoutAt)
		}
		result = append(result, QueueMessageCo{
			ID:             messages[i].ID,
			Queue:          messages[i].Queue,
			BodyPreview:    messages[i].BodyPreview,
			Created:        messages[i].Created,
			Updated:        messages[i].Updated,
			Timeout:        messages[i].Timeout,
			Received:       messages[i].Received,
			Priority:       messages[i].Priority,
			State:          state,
			AgeSec:         queueMessageAgeSec(now, createdAt, hasCreated),
			AvailableInSec: queueMessageAvailableInSec(now, timeoutAt, hasTimeout, state),
			NextClaimAt:    nextClaimAt,
			MaxReceive:     queue.DefaultMaxReceive,
		})
	}
	return result, nil
}

func GetQueueSummary(ctx fwusecase.Context, qry QueueSummaryQry) (QueueSummaryCo, error) {
	messages, err := models.ListQueueMessageSnapshots(ctx.Std(), strings.TrimSpace(qry.Queue))
	if err != nil {
		return QueueSummaryCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load queue summary", err)
	}

	now := timefmt.NowUTC()
	byQueue := make(map[string]*QueueSummaryItemCo)
	order := make([]string, 0)
	for i := range messages {
		item := byQueue[messages[i].Queue]
		if item == nil {
			item = &QueueSummaryItemCo{Queue: messages[i].Queue}
			byQueue[messages[i].Queue] = item
			order = append(order, messages[i].Queue)
		}
		item.Total++
		if messages[i].Received > item.MaxReceived {
			item.MaxReceived = messages[i].Received
		}
		if messages[i].Received > 0 && messages[i].Received < queue.DefaultMaxReceive {
			item.Retrying++
		}

		timeoutAt, hasTimeout := parseQueueTime(messages[i].Timeout)
		createdAt, hasCreated := parseQueueTime(messages[i].Created)
		if messages[i].Received >= queue.DefaultMaxReceive {
			item.Exhausted++
			updateOldestAge(&item.OldestExhaustedAgeSec, now, createdAt, hasCreated)
			continue
		}
		if hasTimeout && timeoutAt.After(now) {
			updateNextClaim(item, now, timeoutAt)
			if messages[i].Received > 0 {
				item.InFlight++
				updateOldestAge(&item.OldestInFlightAgeSec, now, createdAt, hasCreated)
				continue
			}
			item.Delayed++
			continue
		}
		item.Ready++
		updateOldestAge(&item.OldestReadyAgeSec, now, createdAt, hasCreated)
	}

	result := QueueSummaryCo{Queues: make([]QueueSummaryItemCo, 0, len(order))}
	for _, queueName := range order {
		result.Queues = append(result.Queues, *byQueue[queueName])
	}
	return result, nil
}

const (
	QueueMessageStateReady     = "ready"
	QueueMessageStateDelayed   = "delayed"
	QueueMessageStateInFlight  = "in_flight"
	QueueMessageStateExhausted = "exhausted"
)

func normalizeQueueMessageState(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	switch value {
	case QueueMessageStateReady, QueueMessageStateDelayed, QueueMessageStateInFlight, QueueMessageStateExhausted:
		return value, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "queue message state is invalid", nil)
	}
}

func classifyQueueMessage(received int, timeout string, now time.Time) string {
	timeoutAt, hasTimeout := parseQueueTime(timeout)
	if received >= queue.DefaultMaxReceive {
		return QueueMessageStateExhausted
	}
	if hasTimeout && timeoutAt.After(now) {
		if received > 0 {
			return QueueMessageStateInFlight
		}
		return QueueMessageStateDelayed
	}
	return QueueMessageStateReady
}

func GetQueueMessageCorrelation(ctx fwusecase.Context, qry QueueMessageCorrelationQry) (QueueMessageCorrelationCo, error) {
	messageID := strings.TrimSpace(qry.MessageID)
	if messageID == "" {
		return QueueMessageCorrelationCo{}, fwusecase.E(fwusecase.CodeValidation, "message ID is required", nil)
	}

	refs, err := models.ListQueueMessageReferences(ctx.Std(), messageID)
	if err != nil {
		return QueueMessageCorrelationCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load queue message correlation", err)
	}

	result := QueueMessageCorrelationCo{
		MessageID:  messageID,
		References: make([]QueueMessageReferenceCo, 0, len(refs)),
	}
	for i := range refs {
		result.References = append(result.References, QueueMessageReferenceCo{
			Type:        refs[i].Type,
			ID:          refs[i].ID,
			Status:      refs[i].Status,
			RelatedType: refs[i].RelatedType,
			RelatedID:   refs[i].RelatedID,
		})
	}
	return result, nil
}

func RetryQueueMessageNow(ctx fwusecase.Context, cmd QueueMessageActionCmd) error {
	messageID, queueName, err := normalizeQueueMessageAction(cmd)
	if err != nil {
		return err
	}
	if DefaultQueueManager == nil {
		return fwusecase.E(fwusecase.CodeInternal, "queue manager is not configured", nil)
	}

	if err := DefaultQueueManager.RetryMessageNow(ctx.Std(), queueName, messageID); err != nil {
		return queueMessageActionError("retry queue message", err)
	}
	return nil
}

func DeleteQueueMessage(ctx fwusecase.Context, cmd QueueMessageActionCmd) error {
	messageID, queueName, err := normalizeQueueMessageAction(cmd)
	if err != nil {
		return err
	}
	if DefaultQueueManager == nil {
		return fwusecase.E(fwusecase.CodeInternal, "queue manager is not configured", nil)
	}

	if err := DefaultQueueManager.DeleteMessage(ctx.Std(), queueName, messageID); err != nil {
		return queueMessageActionError("delete queue message", err)
	}
	return nil
}

func normalizeQueueMessageAction(cmd QueueMessageActionCmd) (string, string, error) {
	messageID := strings.TrimSpace(cmd.MessageID)
	if messageID == "" {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "message ID is required", nil)
	}
	queueName := strings.TrimSpace(cmd.Queue)
	if queueName == "" {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "queue is required", nil)
	}
	return messageID, queueName, nil
}

func queueMessageActionError(action string, err error) error {
	if errors.Is(err, queue.ErrMessageNotFound) {
		return fwusecase.E(fwusecase.CodeNotFound, "queue message not found", err)
	}
	if errors.Is(err, queue.ErrMessageNotExhausted) {
		return fwusecase.E(fwusecase.CodeConflict, "only exhausted queue messages can be retried", err)
	}
	return fwusecase.E(fwusecase.CodeInternal, "failed to "+action, err)
}

func parseQueueTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		timefmt.SQLiteDateTimeLayout,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func updateOldestAge(target *int64, now time.Time, createdAt time.Time, ok bool) {
	if !ok {
		return
	}
	age := int64(now.Sub(createdAt).Seconds())
	if age < 0 {
		age = 0
	}
	if age > *target {
		*target = age
	}
}

func queueMessageAgeSec(now time.Time, createdAt time.Time, ok bool) int64 {
	if !ok {
		return 0
	}
	age := int64(now.Sub(createdAt).Seconds())
	if age < 0 {
		return 0
	}
	return age
}

func queueMessageAvailableInSec(now time.Time, timeoutAt time.Time, ok bool, state string) int64 {
	if !ok || state == QueueMessageStateReady || state == QueueMessageStateExhausted {
		return 0
	}
	delay := int64(timeoutAt.Sub(now).Seconds())
	if delay < 0 {
		return 0
	}
	return delay
}

func updateNextClaim(item *QueueSummaryItemCo, now time.Time, timeoutAt time.Time) {
	nextClaimInSec := queueMessageAvailableInSec(now, timeoutAt, true, QueueMessageStateDelayed)
	if item.NextClaimAt == "" || nextClaimInSec < item.NextClaimInSec {
		item.NextClaimAt = timefmt.RFC3339(timeoutAt)
		item.NextClaimInSec = nextClaimInSec
	}
}
