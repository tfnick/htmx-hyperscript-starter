package usecase_test

import (
	"context"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

const (
	forumSeedAdminID = "019ea0c1-0001-7000-8000-000000000001"
	forumSeedUserID  = "019ea0c1-0001-7000-8000-000000000002"
	forumSeedOtherID = "019ea0c1-0001-7000-8000-000000000003"
)

func TestListForumCategoriesIncludesDailySeed(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	categories, err := usecase.ListForumCategories(fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI))
	if err != nil {
		t.Fatalf("list forum categories: %v", err)
	}

	if len(categories.Items) == 0 || categories.Items[0].Slug != "daily" {
		t.Fatalf("expected seeded daily category, got %#v", categories.Items)
	}
}

func TestCreateForumThreadAndReplyNotifyThreadAuthor(t *testing.T) {
	manager := setupUsecaseOrderTxDB(t)

	authorCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedUserID, "Li Si", false)
	thread, err := usecase.CreateForumThread(authorCtx, usecase.CreateForumThreadCmd{
		CategorySlug: "daily",
		Title:        "Share a quiet launch note",
		Body:         "The first useful forum thread should be easy to scan.",
	})
	if err != nil {
		t.Fatalf("create forum thread: %v", err)
	}
	if thread.ID == "" || thread.Category.Slug != "daily" || thread.Author.ID != forumSeedUserID {
		t.Fatalf("unexpected created thread: %#v", thread)
	}

	replyCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedOtherID, "Wang Wu", false)
	replied, err := usecase.ReplyForumThread(replyCtx, usecase.ReplyForumThreadCmd{
		ThreadID: thread.ID,
		Body:     "A reply should update activity and notify the author.",
	})
	if err != nil {
		t.Fatalf("reply forum thread: %v", err)
	}
	if replied.ReplyCount != 1 || len(replied.Posts) != 1 || replied.Posts[0].Author.ID != forumSeedOtherID {
		t.Fatalf("unexpected replied thread: %#v", replied)
	}

	appDB, err := manager.GetDB("app")
	if err != nil {
		t.Fatalf("get app db: %v", err)
	}
	var notificationCount int
	if err := appDB.Get(&notificationCount, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = ?
		  AND source_type = 'forum_thread'
		  AND source_id = ?
	`, forumSeedUserID, thread.ID); err != nil {
		t.Fatalf("count forum notifications: %v", err)
	}
	if notificationCount != 1 {
		t.Fatalf("expected one forum reply notification, got %d", notificationCount)
	}
}

func TestForumThreadUpdateRequiresOwnerOrAdmin(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ownerCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedUserID, "Owner", false)
	thread, err := usecase.CreateForumThread(ownerCtx, usecase.CreateForumThreadCmd{
		CategorySlug: "daily",
		Title:        "Permission check",
		Body:         "Only the author or an admin should update this.",
	})
	if err != nil {
		t.Fatalf("create forum thread: %v", err)
	}

	otherCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedOtherID, "Other", false)
	if _, err := usecase.UpdateForumThread(otherCtx, usecase.UpdateForumThreadCmd{
		ID:    thread.ID,
		Title: "Hijacked title",
		Body:  "This should be rejected.",
	}); fwusecase.CodeOf(err) != fwusecase.CodeForbidden {
		t.Fatalf("expected forbidden update, got %v", err)
	}

	adminCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedAdminID, "Admin", true)
	updated, err := usecase.UpdateForumThread(adminCtx, usecase.UpdateForumThreadCmd{
		ID:    thread.ID,
		Title: "Admin title",
		Body:  "Admin can moderate the content.",
	})
	if err != nil {
		t.Fatalf("admin update forum thread: %v", err)
	}
	if updated.Title != "Admin title" {
		t.Fatalf("expected admin update to persist, got %#v", updated)
	}
}

func authenticatedForumUsecaseContext(ctx context.Context, userID string, name string, admin bool) fwusecase.Context {
	callCtx := fwusecase.NewContext(ctx, fwusecase.SurfaceInternalAPI)
	callCtx.Actor = fwusecase.ActorContext{
		Authenticated: true,
		UserID:        userID,
		Name:          name,
		IsAdmin:       admin,
	}
	return callCtx
}
