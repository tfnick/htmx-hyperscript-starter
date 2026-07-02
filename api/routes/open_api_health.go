package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type OpenAPIHealthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Status  string `json:"status"`
		Surface string `json:"surface"`
		Version string `json:"version"`
	} `json:"data"`
}

func GetOpenAPIHealth(c echo.Context) error {
	resp := OpenAPIHealthResponse{Success: true}
	resp.Data.Status = "ok"
	resp.Data.Surface = "open-api"
	resp.Data.Version = "v1"

	return c.JSON(http.StatusOK, resp)
}
