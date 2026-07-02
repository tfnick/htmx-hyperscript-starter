package fwrequest

import (
	"strconv"

	"github.com/labstack/echo/v4"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func PageQuery(c echo.Context) fwusecase.PageQuery {
	return fwusecase.PageQuery{
		Page:     queryPositiveInt(c, "page"),
		PageSize: queryPositiveInt(c, "page_size"),
	}
}

func queryPositiveInt(c echo.Context, name string) int {
	value := c.QueryParam(name)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return -1
	}
	return parsed
}
