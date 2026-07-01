package forum

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/zachatrocity/htmx-hyperscript-starter/api/templates"
)

type Routes struct {
	store     *Store
	templates *templates.Resolver
}

func Register(group *echo.Group, store *Store, resolver *templates.Resolver) {
	routes := &Routes{
		store:     store,
		templates: resolver,
	}

	group.GET("/threads", routes.threadList)
	group.POST("/threads", routes.createThread)
	group.GET("/threads/:id", routes.threadDetail)
	group.POST("/threads/:id/replies", routes.createReply)
}

func (r *Routes) threadList(c echo.Context) error {
	return r.render(c, http.StatusOK, "components/forum/thread-list.html", ThreadListView{
		Threads: r.store.ListThreads(),
	})
}

func (r *Routes) createThread(c echo.Context) error {
	thread, err := r.store.CreateThread(
		c.FormValue("title"),
		c.FormValue("author"),
		c.FormValue("category"),
		c.FormValue("body"),
	)
	if err != nil {
		return r.handleForumError(c, err)
	}

	c.Response().Header().Set("HX-Trigger", "forum:thread-created")
	return r.render(c, http.StatusCreated, "components/forum/thread-detail.html", ThreadDetailView{
		Thread: thread,
	})
}

func (r *Routes) threadDetail(c echo.Context) error {
	thread, err := r.store.GetThread(c.Param("id"))
	if err != nil {
		return r.handleForumError(c, err)
	}

	return r.render(c, http.StatusOK, "components/forum/thread-detail.html", ThreadDetailView{
		Thread: thread,
	})
}

func (r *Routes) createReply(c echo.Context) error {
	thread, err := r.store.AddReply(c.Param("id"), c.FormValue("author"), c.FormValue("body"))
	if err != nil {
		return r.handleForumError(c, err)
	}

	return r.render(c, http.StatusCreated, "components/forum/thread-detail.html", ThreadDetailView{
		Thread: thread,
	})
}

func (r *Routes) handleForumError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrThreadNotFound):
		return r.render(c, http.StatusNotFound, "components/forum/error.html", ErrorView{
			Title:   "Thread not found",
			Message: "That discussion could not be found.",
		})
	case errors.Is(err, ErrInvalidInput):
		return r.render(c, http.StatusBadRequest, "components/forum/error.html", ErrorView{
			Title:   "Missing details",
			Message: "Please include a title and message before posting.",
		})
	default:
		return err
	}
}

func (r *Routes) render(c echo.Context, status int, name string, data any) error {
	var out bytes.Buffer
	if err := r.templates.Execute(&out, name, data); err != nil {
		return err
	}

	return c.HTML(status, out.String())
}

type ThreadListView struct {
	Threads []Thread
}

type ThreadDetailView struct {
	Thread Thread
}

type ErrorView struct {
	Title   string
	Message string
}
