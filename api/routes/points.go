package routes

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/framework/realtime"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type PointsResponse struct {
	UserID  string `json:"user_id"`
	Balance int64  `json:"balance"`
}

func ToPointsResponse(points usecase.PointsCo) PointsResponse {
	return PointsResponse{
		UserID:  points.UserID,
		Balance: points.Balance,
	}
}

func GetMyPoints(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	points, err := usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: currentUser.ID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToPointsResponse(points))
}

func UserRealtimeWebSocket(c echo.Context) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	initialPoints, err := usecase.GetUserPoints(ctx, usecase.PointsBalanceQry{UserID: currentUser.ID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	clientID := strings.TrimSpace(c.QueryParam("client_id"))

	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		OriginPatterns: realtime.WebSocketOriginPatternsFromEnv(),
	})
	if err != nil {
		return nil
	}
	defer conn.CloseNow()

	sub := realtime.SubscribeClient(currentUser.ID, clientID)
	defer sub.Close()

	initialMessage, err := json.Marshal(realtime.NewPointsMessage(realtime.PointsPayload{
		UserID:   initialPoints.UserID,
		ClientID: sub.ClientID(),
		Balance:  initialPoints.Balance,
	}, ""))
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "failed to create points event")
		return nil
	}

	wsCtx := conn.CloseRead(c.Request().Context())
	if err := writeRealtimeWebSocketMessage(wsCtx, conn, initialMessage); err != nil {
		return nil
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case message, ok := <-sub.Messages:
			if !ok {
				return nil
			}
			if err := writeRealtimeWebSocketMessage(wsCtx, conn, message); err != nil {
				return nil
			}
		case <-heartbeat.C:
			if err := conn.Ping(wsCtx); err != nil {
				return nil
			}
		case <-wsCtx.Done():
			return nil
		}
	}
}

func writeRealtimeWebSocketMessage(ctx context.Context, conn *websocket.Conn, payload []byte) error {
	return conn.Write(ctx, websocket.MessageText, payload)
}
