package models_test

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	forumModelSeedUserID  = "019ea0c1-0001-7000-8000-000000000002"
	forumModelSeedOtherID = "019ea0c1-0001-7000-8000-000000000003"
)

func TestListForumThreadsFiltersPrivateThreads(t *testing.T) {
	setupModelsTestDB(t)

	publicThread := insertModelForumThread(t, "Public model thread", models.ForumThreadVisibilityPublic)
	privateThread := insertModelForumThread(t, "Private model thread", models.ForumThreadVisibilityPrivate)

	threads, err := models.ListForumThreads(t.Context(), models.ForumThreadQuery{
		CategorySlug: "daily",
		Limit:        10,
	}, "")
	if err != nil {
		t.Fatalf("list forum threads: %v", err)
	}

	if len(threads) != 1 {
		t.Fatalf("expected one public thread, got %#v", threads)
	}
	if threads[0].ID != publicThread.ID {
		t.Fatalf("expected public thread %s, got %#v", publicThread.ID, threads[0])
	}
	if threads[0].ID == privateThread.ID {
		t.Fatalf("private thread should not be listed: %#v", threads[0])
	}

	total, err := models.CountForumThreads(t.Context(), models.ForumThreadQuery{CategorySlug: "daily"})
	if err != nil {
		t.Fatalf("count forum threads: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected one public thread count, got %d", total)
	}
}

func TestSoftDeleteForumPostRebuildsThreadActivityToPreviousReply(t *testing.T) {
	setupModelsTestDB(t)

	thread := insertModelForumThread(t, "Activity rebuild thread", models.ForumThreadVisibilityPublic)
	first := insertModelForumPost(t, thread.ID, forumModelSeedUserID, "First reply remains published.")
	second := insertModelForumPost(t, thread.ID, forumModelSeedOtherID, "Second reply will be deleted.")

	activeThread, err := models.GetForumThreadByID(t.Context(), thread.ID)
	if err != nil {
		t.Fatalf("get active thread: %v", err)
	}
	if activeThread.ReplyCount != 2 || activeThread.LastPostID != second.ID {
		t.Fatalf("expected second reply as latest before delete, got %#v", activeThread)
	}

	if err := models.SoftDeleteForumPost(t.Context(), second.ID); err != nil {
		t.Fatalf("delete latest forum post: %v", err)
	}

	rebuilt, err := models.GetForumThreadByID(t.Context(), thread.ID)
	if err != nil {
		t.Fatalf("get rebuilt thread: %v", err)
	}
	if rebuilt.ReplyCount != 1 {
		t.Fatalf("expected reply count rebuilt to 1, got %#v", rebuilt)
	}
	if rebuilt.LastPostID != first.ID || rebuilt.LastPostUserID != first.AuthorID || rebuilt.LastPostAt == "" {
		t.Fatalf("expected first reply as latest after delete, first=%#v thread=%#v", first, rebuilt)
	}
}

func insertModelForumThread(t *testing.T, title string, visibility string) models.ForumThread {
	t.Helper()

	thread := &models.ForumThread{
		CategoryID: "forum-category-daily",
		AuthorID:   forumModelSeedUserID,
		Title:      title,
		Body:       "Model-level forum thread body.",
		Visibility: visibility,
	}
	if err := models.InsertForumThread(t.Context(), thread); err != nil {
		t.Fatalf("insert forum thread: %v", err)
	}
	return *thread
}

func insertModelForumPost(t *testing.T, threadID string, authorID string, body string) models.ForumPost {
	t.Helper()

	post := &models.ForumPost{
		ThreadID: threadID,
		AuthorID: authorID,
		Body:     body,
	}
	if err := models.InsertForumPost(t.Context(), post); err != nil {
		t.Fatalf("insert forum post: %v", err)
	}
	return *post
}
