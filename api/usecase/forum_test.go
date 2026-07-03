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
	if thread.Visibility != usecase.ForumThreadVisibilityPublic {
		t.Fatalf("expected default public visibility, got %q", thread.Visibility)
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

func TestPrivateForumThreadVisibility(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	authorCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedUserID, "Owner", false)
	privateThread, err := usecase.CreateForumThread(authorCtx, usecase.CreateForumThreadCmd{
		CategorySlug: "daily",
		Title:        "Private planning note",
		Body:         "Only the author and admins should be able to read this thread.",
		Visibility:   usecase.ForumThreadVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("create private forum thread: %v", err)
	}
	if privateThread.Visibility != usecase.ForumThreadVisibilityPrivate {
		t.Fatalf("expected private visibility, got %#v", privateThread)
	}

	anonymousCtx := fwusecase.NewContext(t.Context(), fwusecase.SurfaceInternalAPI)
	listed, err := usecase.ListForumThreads(anonymousCtx, usecase.ForumThreadsQry{CategorySlug: "daily"})
	if err != nil {
		t.Fatalf("list forum threads: %v", err)
	}
	for _, item := range listed.Items {
		if item.ID == privateThread.ID {
			t.Fatalf("private thread should not appear in public list: %#v", listed.Items)
		}
	}

	if _, err := usecase.GetForumThreadDetail(anonymousCtx, usecase.ForumThreadDetailQry{ID: privateThread.ID}); fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected anonymous private thread lookup to be not found, got %v", err)
	}

	otherCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedOtherID, "Other", false)
	if _, err := usecase.GetForumThreadDetail(otherCtx, usecase.ForumThreadDetailQry{ID: privateThread.ID}); fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected other user private thread lookup to be not found, got %v", err)
	}
	if _, err := usecase.ReplyForumThread(otherCtx, usecase.ReplyForumThreadCmd{
		ThreadID: privateThread.ID,
		Body:     "This reply should not be allowed.",
	}); fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected other user private thread reply to be not found, got %v", err)
	}

	authorDetail, err := usecase.GetForumThreadDetail(authorCtx, usecase.ForumThreadDetailQry{ID: privateThread.ID})
	if err != nil {
		t.Fatalf("author get private forum thread: %v", err)
	}
	if authorDetail.ID != privateThread.ID {
		t.Fatalf("expected author to read private thread, got %#v", authorDetail)
	}

	adminCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedAdminID, "Admin", true)
	adminDetail, err := usecase.GetForumThreadDetail(adminCtx, usecase.ForumThreadDetailQry{ID: privateThread.ID})
	if err != nil {
		t.Fatalf("admin get private forum thread: %v", err)
	}
	if adminDetail.ID != privateThread.ID {
		t.Fatalf("expected admin to read private thread, got %#v", adminDetail)
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

func TestUpdateForumPostReturnsReplyBeyondDefaultDetailPage(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ownerCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedUserID, "Owner", false)
	thread, err := usecase.CreateForumThread(ownerCtx, usecase.CreateForumThreadCmd{
		CategorySlug: "daily",
		Title:        "Long reply thread",
		Body:         "This thread has enough replies to cross the default detail page.",
	})
	if err != nil {
		t.Fatalf("create forum thread: %v", err)
	}

	replyCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedOtherID, "Reply Author", false)
	for i := 0; i < fwusecase.DefaultPageSize+1; i++ {
		if _, err := usecase.ReplyForumThread(replyCtx, usecase.ReplyForumThreadCmd{
			ThreadID: thread.ID,
			Body:     "Reply body for pagination regression.",
		}); err != nil {
			t.Fatalf("create reply %d: %v", i+1, err)
		}
	}

	allReplies, err := usecase.GetForumThreadDetail(ownerCtx, usecase.ForumThreadDetailQry{
		ID:       thread.ID,
		PostSize: fwusecase.MaxPageSize,
	})
	if err != nil {
		t.Fatalf("load all replies: %v", err)
	}
	if len(allReplies.Posts) != fwusecase.DefaultPageSize+1 {
		t.Fatalf("expected %d replies, got %d", fwusecase.DefaultPageSize+1, len(allReplies.Posts))
	}
	target := allReplies.Posts[fwusecase.DefaultPageSize]

	updated, err := usecase.UpdateForumPost(replyCtx, usecase.UpdateForumPostCmd{
		ID:   target.ID,
		Body: "Updated reply beyond the first page.",
	})
	if err != nil {
		t.Fatalf("update reply beyond default page: %v", err)
	}
	if updated.ID != target.ID || updated.Body != "Updated reply beyond the first page." {
		t.Fatalf("unexpected updated reply: %#v", updated)
	}
}

func TestGetForumThreadDetailReturnsPaginatedReplies(t *testing.T) {
	setupUsecaseOrderTxDB(t)

	ownerCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedUserID, "Owner", false)
	thread, err := usecase.CreateForumThread(ownerCtx, usecase.CreateForumThreadCmd{
		CategorySlug: "daily",
		Title:        "Paginated replies",
		Body:         "Replies should be loaded one page at a time.",
	})
	if err != nil {
		t.Fatalf("create forum thread: %v", err)
	}

	replyCtx := authenticatedForumUsecaseContext(t.Context(), forumSeedOtherID, "Reply Author", false)
	for i := 1; i <= 5; i++ {
		if _, err := usecase.ReplyForumThread(replyCtx, usecase.ReplyForumThreadCmd{
			ThreadID: thread.ID,
			Body:     "Reply page body",
		}); err != nil {
			t.Fatalf("create reply %d: %v", i, err)
		}
	}

	detail, err := usecase.GetForumThreadDetail(ownerCtx, usecase.ForumThreadDetailQry{
		ID:       thread.ID,
		PostPage: 2,
		PostSize: 2,
	})
	if err != nil {
		t.Fatalf("get forum thread detail: %v", err)
	}
	if len(detail.Posts) != 2 {
		t.Fatalf("expected second reply page to contain 2 replies, got %d", len(detail.Posts))
	}
	if detail.PostPagination.Page != 2 || detail.PostPagination.PageSize != 2 ||
		detail.PostPagination.TotalItems != 5 || detail.PostPagination.TotalPages != 3 ||
		!detail.PostPagination.HasPrevious || !detail.PostPagination.HasNext {
		t.Fatalf("unexpected reply pagination: %#v", detail.PostPagination)
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
