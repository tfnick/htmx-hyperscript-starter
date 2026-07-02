package fwcontext

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const RequestIDContextKey = "request_id"

const RequestIDHeader = "X-Request-ID"

func GetRequestID(c echo.Context) string {
	requestID, ok := c.Get(RequestIDContextKey).(string)
	if !ok {
		return ""
	}
	return requestID
}

func SetRequestID(c echo.Context) string {
	requestID := c.Request().Header.Get(RequestIDHeader)
	if requestID == "" {
		requestID = uuid.Must(uuid.NewV7()).String()
	}
	c.Set(RequestIDContextKey, requestID)
	c.Response().Header().Set(RequestIDHeader, requestID)
	return requestID
}
