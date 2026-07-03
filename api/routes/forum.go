package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type ForumCategoryResponse struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ForumAuthorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ForumThreadSummaryResponse struct {
	ID             string                `json:"id"`
	Category       ForumCategoryResponse `json:"category"`
	Author         ForumAuthorResponse   `json:"author"`
	Title          string                `json:"title"`
	BodyExcerpt    string                `json:"body_excerpt"`
	Status         string                `json:"status"`
	Visibility     string                `json:"visibility"`
	IsPinned       bool                  `json:"is_pinned"`
	IsLocked       bool                  `json:"is_locked"`
	ViewCount      int                   `json:"view_count"`
	ReplyCount     int                   `json:"reply_count"`
	LastPostID     string                `json:"last_post_id"`
	LastPostAuthor ForumAuthorResponse   `json:"last_post_author"`
	LastPostAt     string                `json:"last_post_at"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
}

type ForumPostResponse struct {
	ID        string              `json:"id"`
	ThreadID  string              `json:"thread_id"`
	Author    ForumAuthorResponse `json:"author"`
	Body      string              `json:"body"`
	Status    string              `json:"status"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
}

type ForumThreadDetailResponse struct {
	ID             string                `json:"id"`
	Category       ForumCategoryResponse `json:"category"`
	Author         ForumAuthorResponse   `json:"author"`
	Title          string                `json:"title"`
	Body           string                `json:"body"`
	Status         string                `json:"status"`
	Visibility     string                `json:"visibility"`
	IsPinned       bool                  `json:"is_pinned"`
	IsLocked       bool                  `json:"is_locked"`
	ViewCount      int                   `json:"view_count"`
	ReplyCount     int                   `json:"reply_count"`
	LastPostID     string                `json:"last_post_id"`
	LastPostAuthor ForumAuthorResponse   `json:"last_post_author"`
	LastPostAt     string                `json:"last_post_at"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
	Posts          []ForumPostResponse   `json:"posts"`
	Pagination     PaginationResponse    `json:"pagination"`
}

type ForumCategoriesResponse struct {
	Items []ForumCategoryResponse `json:"items"`
}

type ForumThreadsResponse struct {
	Items      []ForumThreadSummaryResponse `json:"items"`
	Pagination PaginationResponse           `json:"pagination"`
	Sort       string                       `json:"sort"`
	Search     string                       `json:"search"`
	Category   string                       `json:"category"`
}

type CreateForumThreadRequest struct {
	CategorySlug string `json:"category_slug"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Visibility   string `json:"visibility"`
}

type UpdateForumThreadRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type ForumPostRequest struct {
	Body string `json:"body"`
}

func RegisterForumRoutes(api *echo.Group) {
	forum := api.Group("/forum")
	forum.Use(middleware.OptionalAuth())
	forum.GET("/categories", GetForumCategories)
	forum.GET("/threads", ListForumThreads)
	forum.GET("/categories/:slug/threads", ListForumThreads)
	forum.GET("/threads/:id", GetForumThread)

	protected := forum.Group("")
	protected.Use(middleware.RequireAuth())
	protected.POST("/threads", CreateForumThread)
	protected.PUT("/threads/:id", UpdateForumThread)
	protected.DELETE("/threads/:id", DeleteForumThread)
	protected.POST("/threads/:id/posts", ReplyForumThread)
	protected.PUT("/posts/:id", UpdateForumPost)
	protected.DELETE("/posts/:id", DeleteForumPost)
}

func GetForumCategories(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	categories, err := usecase.ListForumCategories(ctx)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToForumCategoriesResponse(categories))
}

func ListForumThreads(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	threads, err := usecase.ListForumThreads(ctx, usecase.ForumThreadsQry{
		CategorySlug: c.Param("slug"),
		Search:       c.QueryParam("q"),
		Sort:         c.QueryParam("sort"),
		Page:         page.Page,
		PageSize:     page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToForumThreadsResponse(threads))
}

func GetForumThread(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	thread, err := usecase.GetForumThreadDetail(ctx, usecase.ForumThreadDetailQry{
		ID:        c.Param("id"),
		CountView: true,
		PostPage:  page.Page,
		PostSize:  page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToForumThreadDetailResponse(thread))
}

func CreateForumThread(c echo.Context) error {
	req, err := bindCreateForumThreadRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	thread, err := usecase.CreateForumThread(ctx, usecase.CreateForumThreadCmd{
		CategorySlug: req.CategorySlug,
		Title:        req.Title,
		Body:         req.Body,
		Visibility:   req.Visibility,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, ToForumThreadDetailResponse(thread))
}

func ReplyForumThread(c echo.Context) error {
	req, err := bindForumPostRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	thread, err := usecase.ReplyForumThread(ctx, usecase.ReplyForumThreadCmd{
		ThreadID: c.Param("id"),
		Body:     req.Body,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Created(c, ToForumThreadDetailResponse(thread))
}

func UpdateForumThread(c echo.Context) error {
	req, err := bindUpdateForumThreadRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	thread, err := usecase.UpdateForumThread(ctx, usecase.UpdateForumThreadCmd{
		ID:    c.Param("id"),
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToForumThreadDetailResponse(thread))
}

func DeleteForumThread(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.DeleteForumThread(ctx, usecase.DeleteForumThreadCmd{ID: c.Param("id")}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Message(c, http.StatusOK, "thread deleted")
}

func UpdateForumPost(c echo.Context) error {
	req, err := bindForumPostRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	post, err := usecase.UpdateForumPost(ctx, usecase.UpdateForumPostCmd{
		ID:   c.Param("id"),
		Body: req.Body,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToForumPostResponse(post))
}

func DeleteForumPost(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.DeleteForumPost(ctx, usecase.DeleteForumPostCmd{ID: c.Param("id")}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.Message(c, http.StatusOK, "reply deleted")
}

func bindCreateForumThreadRequest(c echo.Context) (CreateForumThreadRequest, error) {
	var req CreateForumThreadRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.CategorySlug = c.FormValue("category_slug")
	req.Title = c.FormValue("title")
	req.Body = c.FormValue("body")
	req.Visibility = c.FormValue("visibility")
	return req, nil
}

func bindUpdateForumThreadRequest(c echo.Context) (UpdateForumThreadRequest, error) {
	var req UpdateForumThreadRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Title = c.FormValue("title")
	req.Body = c.FormValue("body")
	return req, nil
}

func bindForumPostRequest(c echo.Context) (ForumPostRequest, error) {
	var req ForumPostRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Body = c.FormValue("body")
	return req, nil
}

func ToForumCategoryResponse(category usecase.ForumCategoryCo) ForumCategoryResponse {
	return ForumCategoryResponse{
		ID:          category.ID,
		Slug:        category.Slug,
		Name:        category.Name,
		Description: category.Description,
		SortOrder:   category.SortOrder,
		Enabled:     category.Enabled,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func ToForumCategoriesResponse(categories usecase.ForumCategoriesCo) ForumCategoriesResponse {
	items := make([]ForumCategoryResponse, 0, len(categories.Items))
	for i := range categories.Items {
		items = append(items, ToForumCategoryResponse(categories.Items[i]))
	}
	return ForumCategoriesResponse{Items: items}
}

func ToForumAuthorResponse(author usecase.ForumAuthorCo) ForumAuthorResponse {
	return ForumAuthorResponse{ID: author.ID, Name: author.Name}
}

func ToForumThreadSummaryResponse(thread usecase.ForumThreadSummaryCo) ForumThreadSummaryResponse {
	return ForumThreadSummaryResponse{
		ID:             thread.ID,
		Category:       ToForumCategoryResponse(thread.Category),
		Author:         ToForumAuthorResponse(thread.Author),
		Title:          thread.Title,
		BodyExcerpt:    thread.BodyExcerpt,
		Status:         thread.Status,
		Visibility:     thread.Visibility,
		IsPinned:       thread.IsPinned,
		IsLocked:       thread.IsLocked,
		ViewCount:      thread.ViewCount,
		ReplyCount:     thread.ReplyCount,
		LastPostID:     thread.LastPostID,
		LastPostAuthor: ToForumAuthorResponse(thread.LastPostAuthor),
		LastPostAt:     thread.LastPostAt,
		CreatedAt:      thread.CreatedAt,
		UpdatedAt:      thread.UpdatedAt,
	}
}

func ToForumThreadsResponse(threads usecase.ForumThreadsCo) ForumThreadsResponse {
	items := make([]ForumThreadSummaryResponse, 0, len(threads.Items))
	for i := range threads.Items {
		items = append(items, ToForumThreadSummaryResponse(threads.Items[i]))
	}
	return ForumThreadsResponse{
		Items:      items,
		Pagination: ToPaginationResponse(threads.Pagination),
		Sort:       threads.Sort,
		Search:     threads.Search,
		Category:   threads.Category,
	}
}

func ToForumPostResponse(post usecase.ForumPostCo) ForumPostResponse {
	return ForumPostResponse{
		ID:        post.ID,
		ThreadID:  post.ThreadID,
		Author:    ToForumAuthorResponse(post.Author),
		Body:      post.Body,
		Status:    post.Status,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}
}

func ToForumPostResponses(posts []usecase.ForumPostCo) []ForumPostResponse {
	items := make([]ForumPostResponse, 0, len(posts))
	for i := range posts {
		items = append(items, ToForumPostResponse(posts[i]))
	}
	return items
}

func ToForumThreadDetailResponse(thread usecase.ForumThreadDetailCo) ForumThreadDetailResponse {
	return ForumThreadDetailResponse{
		ID:             thread.ID,
		Category:       ToForumCategoryResponse(thread.Category),
		Author:         ToForumAuthorResponse(thread.Author),
		Title:          thread.Title,
		Body:           thread.Body,
		Status:         thread.Status,
		Visibility:     thread.Visibility,
		IsPinned:       thread.IsPinned,
		IsLocked:       thread.IsLocked,
		ViewCount:      thread.ViewCount,
		ReplyCount:     thread.ReplyCount,
		LastPostID:     thread.LastPostID,
		LastPostAuthor: ToForumAuthorResponse(thread.LastPostAuthor),
		LastPostAt:     thread.LastPostAt,
		CreatedAt:      thread.CreatedAt,
		UpdatedAt:      thread.UpdatedAt,
		Posts:          ToForumPostResponses(thread.Posts),
		Pagination:     ToPaginationResponse(thread.PostPagination),
	}
}
