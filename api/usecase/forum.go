package usecase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	ForumSortLatestReply = "latest_reply"
	ForumSortLatestPost  = "latest_post"

	ForumNotificationSourceType = "forum_thread"

	ForumThreadVisibilityPublic  = models.ForumThreadVisibilityPublic
	ForumThreadVisibilityPrivate = models.ForumThreadVisibilityPrivate
)

type ForumCategoryCo struct {
	ID          string
	Slug        string
	Name        string
	Description string
	SortOrder   int
	Enabled     bool
	CreatedAt   string
	UpdatedAt   string
}

type ForumAuthorCo struct {
	ID   string
	Name string
}

type ForumThreadSummaryCo struct {
	ID             string
	Category       ForumCategoryCo
	Author         ForumAuthorCo
	Title          string
	BodyExcerpt    string
	Status         string
	Visibility     string
	IsPinned       bool
	IsLocked       bool
	ViewCount      int
	ReplyCount     int
	LastPostID     string
	LastPostAuthor ForumAuthorCo
	LastPostAt     string
	CreatedAt      string
	UpdatedAt      string
}

type ForumPostCo struct {
	ID        string
	ThreadID  string
	Author    ForumAuthorCo
	Body      string
	Status    string
	CreatedAt string
	UpdatedAt string
}

type ForumThreadDetailCo struct {
	ID             string
	Category       ForumCategoryCo
	Author         ForumAuthorCo
	Title          string
	Body           string
	Status         string
	Visibility     string
	IsPinned       bool
	IsLocked       bool
	ViewCount      int
	ReplyCount     int
	LastPostID     string
	LastPostAuthor ForumAuthorCo
	LastPostAt     string
	CreatedAt      string
	UpdatedAt      string
	Posts          []ForumPostCo
}

type ForumCategoriesCo struct {
	Items []ForumCategoryCo
}

type ForumThreadsCo struct {
	Items      []ForumThreadSummaryCo
	Pagination fwusecase.PageResult
	Sort       string
	Search     string
	Category   string
}

type ForumThreadPostsCo struct {
	Items      []ForumPostCo
	Pagination fwusecase.PageResult
}

type ForumThreadsQry struct {
	CategorySlug string
	Search       string
	Sort         string
	Page         int
	PageSize     int
}

type ForumThreadDetailQry struct {
	ID        string
	CountView bool
	PostPage  int
	PostSize  int
}

type CreateForumThreadCmd struct {
	CategorySlug string
	Title        string
	Body         string
	Visibility   string
}

type ReplyForumThreadCmd struct {
	ThreadID string
	Body     string
}

type UpdateForumThreadCmd struct {
	ID    string
	Title string
	Body  string
}

type DeleteForumThreadCmd struct {
	ID string
}

type UpdateForumPostCmd struct {
	ID   string
	Body string
}

type DeleteForumPostCmd struct {
	ID string
}

func ListForumCategories(ctx fwusecase.Context) (ForumCategoriesCo, error) {
	categories, err := models.ListForumCategories(ctx.Std())
	if err != nil {
		return ForumCategoriesCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load forum categories", err)
	}
	return ForumCategoriesCo{Items: forumCategoryCosFromModels(categories)}, nil
}

func ListForumThreads(ctx fwusecase.Context, qry ForumThreadsQry) (ForumThreadsCo, error) {
	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{Page: qry.Page, PageSize: qry.PageSize})
	if err != nil {
		return ForumThreadsCo{}, err
	}

	sort, err := normalizeForumSort(qry.Sort)
	if err != nil {
		return ForumThreadsCo{}, err
	}

	query := models.ForumThreadQuery{
		CategorySlug: strings.TrimSpace(qry.CategorySlug),
		Search:       normalizeLikeQuery(qry.Search),
		Limit:        pageQuery.Limit(),
		Offset:       pageQuery.Offset(),
	}

	total, err := models.CountForumThreads(ctx.Std(), query)
	if err != nil {
		return ForumThreadsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count forum threads", err)
	}

	threads, err := models.ListForumThreads(ctx.Std(), query, sort)
	if err != nil {
		return ForumThreadsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load forum threads", err)
	}

	return ForumThreadsCo{
		Items:      forumThreadSummaryCosFromModels(threads),
		Pagination: fwusecase.NewPageResult(pageQuery, total),
		Sort:       sort,
		Search:     strings.TrimSpace(qry.Search),
		Category:   strings.TrimSpace(qry.CategorySlug),
	}, nil
}

func GetForumThreadDetail(ctx fwusecase.Context, qry ForumThreadDetailQry) (ForumThreadDetailCo, error) {
	threadID := strings.TrimSpace(qry.ID)
	if threadID == "" {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "thread ID is required", nil)
	}

	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{Page: qry.PostPage, PageSize: qry.PostSize})
	if err != nil {
		return ForumThreadDetailCo{}, err
	}

	thread, err := models.GetForumThreadDetail(ctx.Std(), threadID)
	if err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "thread not found", err)
		}
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load thread", err)
	}
	if !canViewForumThread(ctx, thread.AuthorID, thread.Visibility) {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
	}
	if qry.CountView {
		if err := models.IncrementForumThreadViewCount(ctx.Std(), threadID); err != nil {
			return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update thread views", err)
		}
		thread.ViewCount++
	}

	posts, err := models.ListForumPosts(ctx.Std(), models.ForumPostQuery{
		ThreadID: threadID,
		Limit:    pageQuery.Limit(),
		Offset:   pageQuery.Offset(),
	})
	if err != nil {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load thread replies", err)
	}

	return forumThreadDetailCoFromModel(thread, posts), nil
}

func CreateForumThread(ctx fwusecase.Context, cmd CreateForumThreadCmd) (ForumThreadDetailCo, error) {
	if err := requireForumActor(ctx); err != nil {
		return ForumThreadDetailCo{}, err
	}

	categorySlug := strings.TrimSpace(cmd.CategorySlug)
	if categorySlug == "" {
		categorySlug = "daily"
	}
	title, body, err := normalizeForumThreadInput(cmd.Title, cmd.Body)
	if err != nil {
		return ForumThreadDetailCo{}, err
	}
	visibility, err := normalizeForumThreadVisibility(cmd.Visibility)
	if err != nil {
		return ForumThreadDetailCo{}, err
	}

	var threadID string
	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		category, err := models.GetForumCategoryBySlug(txCtx.Std(), categorySlug)
		if err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "category not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load forum category", err)
		}
		thread := &models.ForumThread{
			CategoryID: category.ID,
			AuthorID:   strings.TrimSpace(txCtx.Actor.UserID),
			Title:      title,
			Body:       body,
			Visibility: visibility,
		}
		if err := models.InsertForumThread(txCtx.Std(), thread); err != nil {
			return fwusecase.E(fwusecase.CodeInternal, "failed to create thread", err)
		}
		threadID = thread.ID
		return nil
	})
	if err != nil {
		return ForumThreadDetailCo{}, err
	}
	return GetForumThreadDetail(ctx, ForumThreadDetailQry{ID: threadID})
}

func ReplyForumThread(ctx fwusecase.Context, cmd ReplyForumThreadCmd) (ForumThreadDetailCo, error) {
	if err := requireForumActor(ctx); err != nil {
		return ForumThreadDetailCo{}, err
	}

	threadID := strings.TrimSpace(cmd.ThreadID)
	if threadID == "" {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "thread ID is required", nil)
	}
	body, err := normalizeForumBody(cmd.Body, "reply")
	if err != nil {
		return ForumThreadDetailCo{}, err
	}

	err = fwusecase.WithAppTx(ctx, func(txCtx fwusecase.Context) error {
		thread, err := models.GetForumThreadByID(txCtx.Std(), threadID)
		if err != nil {
			if errors.Is(err, modelerror.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				return fwusecase.E(fwusecase.CodeNotFound, "thread not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to load thread", err)
		}
		if thread.Status != models.ForumContentStatusPublished {
			return fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
		}
		if !canViewForumThread(txCtx, thread.AuthorID, thread.Visibility) {
			return fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
		}
		if thread.IsLocked == 1 {
			return fwusecase.E(fwusecase.CodeConflict, "thread is locked", nil)
		}

		post := &models.ForumPost{
			ThreadID: thread.ID,
			AuthorID: strings.TrimSpace(txCtx.Actor.UserID),
			Body:     body,
		}
		if err := models.InsertForumPost(txCtx.Std(), post); err != nil {
			if errors.Is(err, modelerror.ErrNotFound) {
				return fwusecase.E(fwusecase.CodeNotFound, "thread not found", err)
			}
			return fwusecase.E(fwusecase.CodeInternal, "failed to create reply", err)
		}

		if shouldNotifyForumThreadAuthor(txCtx, thread.AuthorID) {
			actorName := firstNonEmptyString(txCtx.Actor.Name, "Someone")
			if err := fwusecase.RegisterAfterCommit(txCtx, func(runCtx context.Context) {
				notifyCtx := txCtx.WithStd(runCtx)
				_, _ = SendNotification(notifyCtx, SendNotificationCmd{
					NotificationType: NotificationTypeRealtime,
					SourceType:       ForumNotificationSourceType,
					SourceID:         thread.ID,
					UserID:           thread.AuthorID,
					Title:            "New reply to your thread",
					Summary:          actorName + " replied to \"" + thread.Title + "\"",
					Payload: map[string]interface{}{
						"thread_id": thread.ID,
						"post_id":   post.ID,
						"status":    "new_reply",
					},
				})
			}); err != nil {
				return fwusecase.E(fwusecase.CodeInternal, "failed to register reply notification", err)
			}
		}
		return nil
	})
	if err != nil {
		return ForumThreadDetailCo{}, err
	}
	return GetForumThreadDetail(ctx, ForumThreadDetailQry{ID: threadID})
}

func UpdateForumThread(ctx fwusecase.Context, cmd UpdateForumThreadCmd) (ForumThreadDetailCo, error) {
	if err := requireForumActor(ctx); err != nil {
		return ForumThreadDetailCo{}, err
	}

	threadID := strings.TrimSpace(cmd.ID)
	if threadID == "" {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeValidation, "thread ID is required", nil)
	}
	title, body, err := normalizeForumThreadInput(cmd.Title, cmd.Body)
	if err != nil {
		return ForumThreadDetailCo{}, err
	}

	thread, err := models.GetForumThreadByID(ctx.Std(), threadID)
	if err != nil {
		return ForumThreadDetailCo{}, forumNotFoundOrInternal(err, "thread not found", "failed to load thread")
	}
	if err := requireForumContentManager(ctx, thread.AuthorID); err != nil {
		return ForumThreadDetailCo{}, err
	}
	if thread.Status != models.ForumContentStatusPublished {
		return ForumThreadDetailCo{}, fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
	}
	if err := models.UpdateForumThread(ctx.Std(), threadID, title, body); err != nil {
		return ForumThreadDetailCo{}, forumNotFoundOrInternal(err, "thread not found", "failed to update thread")
	}
	return GetForumThreadDetail(ctx, ForumThreadDetailQry{ID: threadID})
}

func DeleteForumThread(ctx fwusecase.Context, cmd DeleteForumThreadCmd) error {
	if err := requireForumActor(ctx); err != nil {
		return err
	}

	threadID := strings.TrimSpace(cmd.ID)
	if threadID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "thread ID is required", nil)
	}
	thread, err := models.GetForumThreadByID(ctx.Std(), threadID)
	if err != nil {
		return forumNotFoundOrInternal(err, "thread not found", "failed to load thread")
	}
	if err := requireForumContentManager(ctx, thread.AuthorID); err != nil {
		return err
	}
	if err := models.SoftDeleteForumThread(ctx.Std(), threadID); err != nil {
		return forumNotFoundOrInternal(err, "thread not found", "failed to delete thread")
	}
	return nil
}

func UpdateForumPost(ctx fwusecase.Context, cmd UpdateForumPostCmd) (ForumPostCo, error) {
	if err := requireForumActor(ctx); err != nil {
		return ForumPostCo{}, err
	}

	postID := strings.TrimSpace(cmd.ID)
	if postID == "" {
		return ForumPostCo{}, fwusecase.E(fwusecase.CodeValidation, "post ID is required", nil)
	}
	body, err := normalizeForumBody(cmd.Body, "reply")
	if err != nil {
		return ForumPostCo{}, err
	}
	post, err := models.GetForumPostByID(ctx.Std(), postID)
	if err != nil {
		return ForumPostCo{}, forumPostNotFoundOrInternal(err)
	}
	thread, err := models.GetForumThreadByID(ctx.Std(), post.ThreadID)
	if err != nil {
		return ForumPostCo{}, forumNotFoundOrInternal(err, "thread not found", "failed to load thread")
	}
	if thread.Status != models.ForumContentStatusPublished || !canViewForumThread(ctx, thread.AuthorID, thread.Visibility) {
		return ForumPostCo{}, fwusecase.E(fwusecase.CodeNotFound, "thread not found", nil)
	}
	if err := requireForumContentManager(ctx, post.AuthorID); err != nil {
		return ForumPostCo{}, err
	}
	if err := models.UpdateForumPost(ctx.Std(), postID, body); err != nil {
		return ForumPostCo{}, forumPostNotFoundOrInternal(err)
	}
	updated, err := models.GetForumPostListItemByID(ctx.Std(), post.ID)
	if err != nil {
		return ForumPostCo{}, forumPostNotFoundOrInternal(err)
	}
	return forumPostCoFromModel(updated), nil
}

func DeleteForumPost(ctx fwusecase.Context, cmd DeleteForumPostCmd) error {
	if err := requireForumActor(ctx); err != nil {
		return err
	}

	postID := strings.TrimSpace(cmd.ID)
	if postID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "post ID is required", nil)
	}
	post, err := models.GetForumPostByID(ctx.Std(), postID)
	if err != nil {
		return forumPostNotFoundOrInternal(err)
	}
	if err := requireForumContentManager(ctx, post.AuthorID); err != nil {
		return err
	}
	if err := models.SoftDeleteForumPost(ctx.Std(), postID); err != nil {
		return forumPostNotFoundOrInternal(err)
	}
	return nil
}

func normalizeForumSort(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", ForumSortLatestReply:
		return ForumSortLatestReply, nil
	case ForumSortLatestPost:
		return ForumSortLatestPost, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "forum sort is invalid", nil)
	}
}

func normalizeForumThreadInput(title string, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "title is required", nil)
	}
	if len([]rune(title)) > 160 {
		return "", "", fwusecase.E(fwusecase.CodeValidation, "title is too long", nil)
	}
	normalizedBody, err := normalizeForumBody(body, "thread body")
	if err != nil {
		return "", "", err
	}
	return title, normalizedBody, nil
}

func normalizeForumThreadVisibility(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", ForumThreadVisibilityPublic:
		return ForumThreadVisibilityPublic, nil
	case ForumThreadVisibilityPrivate:
		return ForumThreadVisibilityPrivate, nil
	default:
		return "", fwusecase.E(fwusecase.CodeValidation, "thread visibility is invalid", nil)
	}
}

func normalizeForumBody(value string, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fwusecase.E(fwusecase.CodeValidation, label+" is required", nil)
	}
	if len([]rune(value)) > 20000 {
		return "", fwusecase.E(fwusecase.CodeValidation, label+" is too long", nil)
	}
	return value, nil
}

func requireForumActor(ctx fwusecase.Context) error {
	if !ctx.Actor.Authenticated || strings.TrimSpace(ctx.Actor.UserID) == "" {
		return fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	return nil
}

func requireForumContentManager(ctx fwusecase.Context, authorID string) error {
	if err := requireForumActor(ctx); err != nil {
		return err
	}
	if ctx.Actor.IsAdmin || strings.TrimSpace(ctx.Actor.UserID) == strings.TrimSpace(authorID) {
		return nil
	}
	return fwusecase.E(fwusecase.CodeForbidden, "cannot manage another user's content", nil)
}

func canViewForumThread(ctx fwusecase.Context, authorID string, visibility string) bool {
	if visibility != ForumThreadVisibilityPrivate {
		return true
	}
	if !ctx.Actor.Authenticated {
		return false
	}
	if ctx.Actor.IsAdmin {
		return true
	}
	return strings.TrimSpace(ctx.Actor.UserID) == strings.TrimSpace(authorID)
}

func shouldNotifyForumThreadAuthor(ctx fwusecase.Context, authorID string) bool {
	return strings.TrimSpace(authorID) != "" && strings.TrimSpace(authorID) != strings.TrimSpace(ctx.Actor.UserID)
}

func forumNotFoundOrInternal(err error, notFoundMessage string, internalMessage string) error {
	if errors.Is(err, modelerror.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return fwusecase.E(fwusecase.CodeNotFound, notFoundMessage, err)
	}
	return fwusecase.E(fwusecase.CodeInternal, internalMessage, err)
}

func forumPostNotFoundOrInternal(err error) error {
	return forumNotFoundOrInternal(err, "reply not found", "failed to load reply")
}

func forumCategoryCoFromModel(category models.ForumCategory) ForumCategoryCo {
	return ForumCategoryCo{
		ID:          category.ID,
		Slug:        category.Slug,
		Name:        category.Name,
		Description: category.Description,
		SortOrder:   category.SortOrder,
		Enabled:     category.Enabled == 1,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func forumCategoryCosFromModels(categories []models.ForumCategory) []ForumCategoryCo {
	items := make([]ForumCategoryCo, 0, len(categories))
	for i := range categories {
		items = append(items, forumCategoryCoFromModel(categories[i]))
	}
	return items
}

func forumThreadSummaryCoFromModel(thread models.ForumThreadListItem) ForumThreadSummaryCo {
	return ForumThreadSummaryCo{
		ID: thread.ID,
		Category: ForumCategoryCo{
			ID:   thread.CategoryID,
			Slug: thread.CategorySlug,
			Name: thread.CategoryName,
		},
		Author:         ForumAuthorCo{ID: thread.AuthorID, Name: thread.AuthorName},
		Title:          thread.Title,
		BodyExcerpt:    forumExcerpt(thread.Body),
		Status:         thread.Status,
		Visibility:     thread.Visibility,
		IsPinned:       thread.IsPinned == 1,
		IsLocked:       thread.IsLocked == 1,
		ViewCount:      thread.ViewCount,
		ReplyCount:     thread.ReplyCount,
		LastPostID:     thread.LastPostID,
		LastPostAuthor: ForumAuthorCo{ID: thread.LastPostUserID, Name: thread.LastPostAuthorName},
		LastPostAt:     thread.LastPostAt,
		CreatedAt:      thread.CreatedAt,
		UpdatedAt:      thread.UpdatedAt,
	}
}

func forumThreadSummaryCosFromModels(threads []models.ForumThreadListItem) []ForumThreadSummaryCo {
	items := make([]ForumThreadSummaryCo, 0, len(threads))
	for i := range threads {
		items = append(items, forumThreadSummaryCoFromModel(threads[i]))
	}
	return items
}

func forumThreadDetailCoFromModel(thread models.ForumThreadDetail, posts []models.ForumPostListItem) ForumThreadDetailCo {
	return ForumThreadDetailCo{
		ID: thread.ID,
		Category: ForumCategoryCo{
			ID:   thread.CategoryID,
			Slug: thread.CategorySlug,
			Name: thread.CategoryName,
		},
		Author:         ForumAuthorCo{ID: thread.AuthorID, Name: thread.AuthorName},
		Title:          thread.Title,
		Body:           thread.Body,
		Status:         thread.Status,
		Visibility:     thread.Visibility,
		IsPinned:       thread.IsPinned == 1,
		IsLocked:       thread.IsLocked == 1,
		ViewCount:      thread.ViewCount,
		ReplyCount:     thread.ReplyCount,
		LastPostID:     thread.LastPostID,
		LastPostAuthor: ForumAuthorCo{ID: thread.LastPostUserID, Name: thread.LastPostAuthorName},
		LastPostAt:     thread.LastPostAt,
		CreatedAt:      thread.CreatedAt,
		UpdatedAt:      thread.UpdatedAt,
		Posts:          forumPostCosFromModels(posts),
	}
}

func forumPostCosFromModels(posts []models.ForumPostListItem) []ForumPostCo {
	items := make([]ForumPostCo, 0, len(posts))
	for i := range posts {
		items = append(items, forumPostCoFromModel(posts[i]))
	}
	return items
}

func forumPostCoFromModel(post models.ForumPostListItem) ForumPostCo {
	return ForumPostCo{
		ID:        post.ID,
		ThreadID:  post.ThreadID,
		Author:    ForumAuthorCo{ID: post.AuthorID, Name: post.AuthorName},
		Body:      post.Body,
		Status:    post.Status,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func forumExcerpt(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= 180 {
		return value
	}
	return string(runes[:180]) + "..."
}

func forumPayloadJSON(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
