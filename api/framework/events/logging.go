package events

import (
	"github.com/rs/zerolog"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
)

func eventLogger() zerolog.Logger {
	return logging.For("events")
}

func logDuplicateSubscription(err error, sub Subscription) {
	logger := eventLogger()
	logSubscription(logger.Error(), sub).
		Err(err).
		Str("status", "rejected").
		Msg("duplicate domain event subscription rejected")
}

func logEventValidationError(err error, sub Subscription) {
	logger := eventLogger()
	logSubscription(logger.Error(), sub).
		Err(err).
		Str("status", "rejected").
		Msg("domain event subscription validation failed")
}

func logPublishValidationError(err error, event Event) {
	logger := eventLogger()
	logEvent(logger.Error(), Subscription{Topic: event.Topic}, event).
		Err(err).
		Str("status", "rejected").
		Msg("domain event publish validation failed")
}

func logEvent(e *zerolog.Event, sub Subscription, event Event) *zerolog.Event {
	return logSubscription(e, sub).
		Str("event_id", event.ID).
		Str("aggregate_type", event.AggregateType).
		Str("aggregate_id", event.AggregateID).
		Time("occurred_at", event.OccurredAt)
}

func logSubscription(e *zerolog.Event, sub Subscription) *zerolog.Event {
	return e.
		Str("topic", sub.Topic).
		Str("subscriber", sub.Subscriber)
}
