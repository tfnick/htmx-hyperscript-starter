package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/db"
)

// CountRecentConversationsByIP counts the number of conversations created from the given IP hash
// since the specified time, used for rate limiting new conversation creation.
func CountRecentConversationsByIP(ctx context.Context, ipHash, since string) (int, error) {
	if strings.TrimSpace(ipHash) == "" {
		return 0, nil
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}
	var count int
	err = d.GetP(&count, `
		SELECT COUNT(1)
		FROM support_conversations
		WHERE visitor_ip_hash = ? AND created_at >= ?`,
		ipHash, since)
	return count, err
}

// CountRecentConversationMessagesByVisitor counts the number of visitor messages sent
// in conversations belonging to the given visitor token hash since the specified time.
func CountRecentConversationMessagesByVisitor(ctx context.Context, tokenHash, since string) (int, error) {
	if strings.TrimSpace(tokenHash) == "" {
		return 0, nil
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}
	var count int
	err = d.GetP(&count, `
		SELECT COUNT(1)
		FROM support_messages m
		JOIN support_conversations c ON c.id = m.conversation_id
		WHERE c.visitor_token_hash = ? AND m.role = ? AND m.created_at >= ?`,
		tokenHash, SupportMessageRoleVisitor, since)
	return count, err
}
