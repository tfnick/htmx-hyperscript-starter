package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

const (
	ForumContentStatusPublished = "published"
	ForumContentStatusDeleted   = "deleted"

	ForumThreadVisibilityPublic  = "public"
	ForumThreadVisibilityPrivate = "private"
)

type ForumCategory struct {
	ID          string `db:"id"`
	Slug        string `db:"slug"`
	Name        string `db:"name"`
	Description string `db:"description"`
	SortOrder   int    `db:"sort_order"`
	Enabled     int    `db:"enabled"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

type ForumThread struct {
	ID             string `db:"id"`
	CategoryID     string `db:"category_id"`
	AuthorID       string `db:"author_id"`
	Title          string `db:"title"`
	Body           string `db:"body"`
	Status         string `db:"status"`
	Visibility     string `db:"visibility"`
	IsPinned       int    `db:"is_pinned"`
	IsLocked       int    `db:"is_locked"`
	ViewCount      int    `db:"view_count"`
	ReplyCount     int    `db:"reply_count"`
	LastPostID     string `db:"last_post_id"`
	LastPostUserID string `db:"last_post_user_id"`
	LastPostAt     string `db:"last_post_at"`
	DeletedAt      string `db:"deleted_at"`
	CreatedAt      string `db:"created_at"`
	UpdatedAt      string `db:"updated_at"`
}

type ForumThreadListItem struct {
	ID                 string `db:"id"`
	CategoryID         string `db:"category_id"`
	CategorySlug       string `db:"category_slug"`
	CategoryName       string `db:"category_name"`
	AuthorID           string `db:"author_id"`
	AuthorName         string `db:"author_name"`
	Title              string `db:"title"`
	Body               string `db:"body"`
	Status             string `db:"status"`
	Visibility         string `db:"visibility"`
	IsPinned           int    `db:"is_pinned"`
	IsLocked           int    `db:"is_locked"`
	ViewCount          int    `db:"view_count"`
	ReplyCount         int    `db:"reply_count"`
	LastPostID         string `db:"last_post_id"`
	LastPostUserID     string `db:"last_post_user_id"`
	LastPostAuthorName string `db:"last_post_author_name"`
	LastPostAt         string `db:"last_post_at"`
	CreatedAt          string `db:"created_at"`
	UpdatedAt          string `db:"updated_at"`
}

type ForumThreadDetail struct {
	ID                 string `db:"id"`
	CategoryID         string `db:"category_id"`
	CategorySlug       string `db:"category_slug"`
	CategoryName       string `db:"category_name"`
	AuthorID           string `db:"author_id"`
	AuthorName         string `db:"author_name"`
	Title              string `db:"title"`
	Body               string `db:"body"`
	Status             string `db:"status"`
	Visibility         string `db:"visibility"`
	IsPinned           int    `db:"is_pinned"`
	IsLocked           int    `db:"is_locked"`
	ViewCount          int    `db:"view_count"`
	ReplyCount         int    `db:"reply_count"`
	LastPostID         string `db:"last_post_id"`
	LastPostUserID     string `db:"last_post_user_id"`
	LastPostAuthorName string `db:"last_post_author_name"`
	LastPostAt         string `db:"last_post_at"`
	CreatedAt          string `db:"created_at"`
	UpdatedAt          string `db:"updated_at"`
}

type ForumPost struct {
	ID        string `db:"id"`
	ThreadID  string `db:"thread_id"`
	AuthorID  string `db:"author_id"`
	Body      string `db:"body"`
	Status    string `db:"status"`
	DeletedAt string `db:"deleted_at"`
	CreatedAt string `db:"created_at"`
	UpdatedAt string `db:"updated_at"`
}

type ForumPostListItem struct {
	ID         string `db:"id"`
	ThreadID   string `db:"thread_id"`
	AuthorID   string `db:"author_id"`
	AuthorName string `db:"author_name"`
	Body       string `db:"body"`
	Status     string `db:"status"`
	CreatedAt  string `db:"created_at"`
	UpdatedAt  string `db:"updated_at"`
}

type ForumThreadQuery struct {
	CategorySlug string `db:"category_slug"`
	Search       string `db:"search"`
	Limit        int    `db:"limit"`
	Offset       int    `db:"offset"`
}

type ForumPostQuery struct {
	ThreadID string `db:"thread_id"`
	Limit    int    `db:"limit"`
	Offset   int    `db:"offset"`
}

func ListForumCategories(ctx context.Context) ([]ForumCategory, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var categories []ForumCategory
	if err := d.SelectP(&categories, `
		SELECT id, slug, name, description, sort_order, enabled, created_at, updated_at
		FROM forum_categories
		WHERE enabled = 1
		ORDER BY sort_order ASC, slug ASC
	`); err != nil {
		return nil, fmt.Errorf("list forum categories failed: %w", err)
	}
	return categories, nil
}

func GetForumCategoryBySlug(ctx context.Context, slug string) (ForumCategory, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ForumCategory{}, fmt.Errorf("database unavailable: %w", err)
	}

	var category ForumCategory
	if err := d.GetP(&category, `
		SELECT id, slug, name, description, sort_order, enabled, created_at, updated_at
		FROM forum_categories
		WHERE slug = ? AND enabled = 1
		LIMIT 1
	`, slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ForumCategory{}, fmt.Errorf("forum category not found: %w", modelerror.ErrNotFound)
		}
		return ForumCategory{}, fmt.Errorf("get forum category failed: %w", err)
	}
	return category, nil
}

func CountForumThreads(ctx context.Context, query ForumThreadQuery) (int, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return 0, fmt.Errorf("database unavailable: %w", err)
	}

	var total int
	if err := d.Get(&total, `
		SELECT COUNT(*)
		FROM forum_threads t
		JOIN forum_categories c ON c.id = t.category_id
		WHERE t.status = 'published'
			AND t.visibility = 'public'
			#[ AND c.slug = :category_slug ]
			#[ AND (LOWER(t.title) LIKE LOWER(:search) ESCAPE '\' OR LOWER(t.body) LIKE LOWER(:search) ESCAPE '\') ]
	`, query); err != nil {
		return 0, fmt.Errorf("count forum threads failed: %w", err)
	}
	return total, nil
}

func ListForumThreads(ctx context.Context, query ForumThreadQuery, sort string) ([]ForumThreadListItem, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	orderSQL := "t.is_pinned DESC, COALESCE(t.last_post_at, t.created_at) DESC, t.id DESC"
	if sort == "latest_post" {
		orderSQL = "t.is_pinned DESC, t.created_at DESC, t.id DESC"
	}

	sqlText := forumThreadListSelectSQL() + `
		WHERE t.status = 'published'
			AND t.visibility = 'public'
			#[ AND c.slug = :category_slug ]
			#[ AND (LOWER(t.title) LIKE LOWER(:search) ESCAPE '\' OR LOWER(t.body) LIKE LOWER(:search) ESCAPE '\') ]
		ORDER BY ` + orderSQL + `
		LIMIT :limit OFFSET :offset
	`
	var threads []ForumThreadListItem
	if err := d.Select(&threads, sqlText, query); err != nil {
		return nil, fmt.Errorf("list forum threads failed: %w", err)
	}
	return threads, nil
}

func InsertForumThread(ctx context.Context, thread *ForumThread) error {
	if thread.ID == "" {
		thread.ID = uuid.Must(uuid.NewV7()).String()
	}
	if thread.Status == "" {
		thread.Status = ForumContentStatusPublished
	}
	if thread.Visibility == "" {
		thread.Visibility = ForumThreadVisibilityPublic
	}
	now := timefmt.NowSQLiteDateTime()
	thread.CreatedAt = now
	thread.UpdatedAt = now

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO forum_threads (
			id, category_id, author_id, title, body, status, visibility, is_pinned, is_locked,
			view_count, reply_count, last_post_id, last_post_user_id, last_post_at,
			created_at, updated_at
		) VALUES (
			:id, :category_id, :author_id, :title, :body, :status, :visibility, :is_pinned, :is_locked,
			:view_count, :reply_count, :last_post_id, :last_post_user_id, NULLIF(:last_post_at, ''),
			:created_at, :updated_at
		)
	`, thread); err != nil {
		return fmt.Errorf("insert forum thread failed: %w", err)
	}
	return nil
}

func GetForumThreadByID(ctx context.Context, id string) (ForumThread, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ForumThread{}, fmt.Errorf("database unavailable: %w", err)
	}

	var thread ForumThread
	if err := d.GetP(&thread, forumThreadRawSelectSQL()+` WHERE id = ? LIMIT 1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ForumThread{}, fmt.Errorf("forum thread not found: %w", modelerror.ErrNotFound)
		}
		return ForumThread{}, fmt.Errorf("get forum thread failed: %w", err)
	}
	return thread, nil
}

func GetForumThreadDetail(ctx context.Context, id string) (ForumThreadDetail, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ForumThreadDetail{}, fmt.Errorf("database unavailable: %w", err)
	}

	var thread ForumThreadDetail
	if err := d.GetP(&thread, forumThreadDetailSelectSQL()+`
		WHERE t.id = ? AND t.status = 'published'
		LIMIT 1
	`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ForumThreadDetail{}, fmt.Errorf("forum thread not found: %w", modelerror.ErrNotFound)
		}
		return ForumThreadDetail{}, fmt.Errorf("get forum thread detail failed: %w", err)
	}
	return thread, nil
}

func IncrementForumThreadViewCount(ctx context.Context, id string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	if _, err := d.ExecP(`UPDATE forum_threads SET view_count = view_count + 1 WHERE id = ? AND status = 'published'`, id); err != nil {
		return fmt.Errorf("increment forum thread views failed: %w", err)
	}
	return nil
}

func UpdateForumThread(ctx context.Context, id string, title string, body string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	result, err := d.ExecP(`
		UPDATE forum_threads
		SET title = ?, body = ?, updated_at = ?
		WHERE id = ? AND status = 'published'
	`, title, body, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return fmt.Errorf("update forum thread failed: %w", err)
	}
	return requireRowsAffected(result, "forum thread not found")
}

func SoftDeleteForumThread(ctx context.Context, id string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	result, err := d.ExecP(`
		UPDATE forum_threads
		SET status = 'deleted', deleted_at = ?, updated_at = ?
		WHERE id = ? AND status = 'published'
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("delete forum thread failed: %w", err)
	}
	return requireRowsAffected(result, "forum thread not found")
}

func InsertForumPost(ctx context.Context, post *ForumPost) error {
	if post.ID == "" {
		post.ID = uuid.Must(uuid.NewV7()).String()
	}
	if post.Status == "" {
		post.Status = ForumContentStatusPublished
	}
	now := timefmt.NowSQLiteDateTime()
	post.CreatedAt = now
	post.UpdatedAt = now

	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if _, err := d.ExecNamed(`
		INSERT INTO forum_posts (id, thread_id, author_id, body, status, created_at, updated_at)
		VALUES (:id, :thread_id, :author_id, :body, :status, :created_at, :updated_at)
	`, post); err != nil {
		return fmt.Errorf("insert forum post failed: %w", err)
	}
	if err := touchForumThreadForPost(ctx, post); err != nil {
		return err
	}
	return nil
}

func ListForumPosts(ctx context.Context, query ForumPostQuery) ([]ForumPostListItem, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var posts []ForumPostListItem
	if err := d.Select(&posts, forumPostListSelectSQL()+`
		WHERE p.thread_id = :thread_id
		  AND p.status = 'published'
		ORDER BY p.created_at ASC, p.id ASC
		LIMIT :limit OFFSET :offset
	`, query); err != nil {
		return nil, fmt.Errorf("list forum posts failed: %w", err)
	}
	return posts, nil
}

func GetForumPostListItemByID(ctx context.Context, id string) (ForumPostListItem, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ForumPostListItem{}, fmt.Errorf("database unavailable: %w", err)
	}

	var post ForumPostListItem
	if err := d.GetP(&post, forumPostListSelectSQL()+`
		WHERE p.id = ? AND p.status = 'published'
		LIMIT 1
	`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ForumPostListItem{}, fmt.Errorf("forum post not found: %w", modelerror.ErrNotFound)
		}
		return ForumPostListItem{}, fmt.Errorf("get forum post list item failed: %w", err)
	}
	return post, nil
}

func GetForumPostByID(ctx context.Context, id string) (ForumPost, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return ForumPost{}, fmt.Errorf("database unavailable: %w", err)
	}

	var post ForumPost
	if err := d.GetP(&post, forumPostRawSelectSQL()+` WHERE id = ? LIMIT 1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ForumPost{}, fmt.Errorf("forum post not found: %w", modelerror.ErrNotFound)
		}
		return ForumPost{}, fmt.Errorf("get forum post failed: %w", err)
	}
	return post, nil
}

func UpdateForumPost(ctx context.Context, id string, body string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	result, err := d.ExecP(`
		UPDATE forum_posts
		SET body = ?, updated_at = ?
		WHERE id = ? AND status = 'published'
	`, body, timefmt.NowSQLiteDateTime(), id)
	if err != nil {
		return fmt.Errorf("update forum post failed: %w", err)
	}
	return requireRowsAffected(result, "forum post not found")
}

func SoftDeleteForumPost(ctx context.Context, id string) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	now := timefmt.NowSQLiteDateTime()
	result, err := d.ExecP(`
		UPDATE forum_posts
		SET status = 'deleted', deleted_at = ?, updated_at = ?
		WHERE id = ? AND status = 'published'
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("delete forum post failed: %w", err)
	}
	if err := requireRowsAffected(result, "forum post not found"); err != nil {
		return err
	}
	if err := rebuildForumThreadActivity(ctx, id); err != nil {
		return err
	}
	return nil
}

func touchForumThreadForPost(ctx context.Context, post *ForumPost) error {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	result, err := d.ExecP(`
		UPDATE forum_threads
		SET reply_count = reply_count + 1,
			last_post_id = ?,
			last_post_user_id = ?,
			last_post_at = ?,
			updated_at = ?
		WHERE id = ? AND status = 'published'
	`, post.ID, post.AuthorID, post.CreatedAt, post.UpdatedAt, post.ThreadID)
	if err != nil {
		return fmt.Errorf("update forum thread activity failed: %w", err)
	}
	return requireRowsAffected(result, "forum thread not found")
}

func rebuildForumThreadActivity(ctx context.Context, postID string) error {
	post, err := GetForumPostByID(ctx, postID)
	if err != nil {
		return err
	}
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}

	var last ForumPost
	lastErr := d.GetP(&last, forumPostRawSelectSQL()+`
		WHERE thread_id = ? AND status = 'published'
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, post.ThreadID)
	if lastErr != nil && !errors.Is(lastErr, sql.ErrNoRows) {
		return fmt.Errorf("load last forum post failed: %w", lastErr)
	}

	replyCount := 0
	if err := d.GetP(&replyCount, `SELECT COUNT(*) FROM forum_posts WHERE thread_id = ? AND status = 'published'`, post.ThreadID); err != nil {
		return fmt.Errorf("count forum posts failed: %w", err)
	}

	lastID, lastUserID, lastAt := "", "", ""
	if lastErr == nil {
		lastID, lastUserID, lastAt = last.ID, last.AuthorID, last.CreatedAt
	}
	if _, err := d.ExecP(`
		UPDATE forum_threads
		SET reply_count = ?, last_post_id = ?, last_post_user_id = ?, last_post_at = NULLIF(?, ''), updated_at = ?
		WHERE id = ?
	`, replyCount, lastID, lastUserID, lastAt, timefmt.NowSQLiteDateTime(), post.ThreadID); err != nil {
		return fmt.Errorf("rebuild forum thread activity failed: %w", err)
	}
	return nil
}

func forumThreadRawSelectSQL() string {
	return `
		SELECT id, category_id, author_id, title, body, status, visibility, is_pinned, is_locked,
			view_count, reply_count, last_post_id, last_post_user_id,
			COALESCE(last_post_at, '') AS last_post_at,
			COALESCE(deleted_at, '') AS deleted_at,
			created_at, updated_at
		FROM forum_threads
	`
}

func forumPostRawSelectSQL() string {
	return `
		SELECT id, thread_id, author_id, body, status,
			COALESCE(deleted_at, '') AS deleted_at,
			created_at, updated_at
		FROM forum_posts
	`
}

func forumThreadListSelectSQL() string {
	return `
		SELECT
			t.id,
			t.category_id,
			c.slug AS category_slug,
			c.name AS category_name,
			t.author_id,
			COALESCE(author.name, '') AS author_name,
			t.title,
			t.body,
			t.status,
			t.visibility,
			t.is_pinned,
			t.is_locked,
			t.view_count,
			t.reply_count,
			t.last_post_id,
			t.last_post_user_id,
			COALESCE(last_author.name, '') AS last_post_author_name,
			COALESCE(t.last_post_at, '') AS last_post_at,
			t.created_at,
			t.updated_at
		FROM forum_threads t
		JOIN forum_categories c ON c.id = t.category_id
		JOIN users author ON author.id = t.author_id
		LEFT JOIN users last_author ON last_author.id = t.last_post_user_id
	`
}

func forumThreadDetailSelectSQL() string {
	return forumThreadListSelectSQL()
}

func forumPostListSelectSQL() string {
	return `
		SELECT
			p.id,
			p.thread_id,
			p.author_id,
			COALESCE(author.name, '') AS author_name,
			p.body,
			p.status,
			p.created_at,
			p.updated_at
		FROM forum_posts p
		JOIN users author ON author.id = p.author_id
	`
}
