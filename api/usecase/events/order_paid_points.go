package events

import (
	"context"
	"errors"
	"fmt"
	"sync"

	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const (
	// OrderPaidTopic 表示“订单已经支付成功”这个领域事实。
	// 发布方只表达事实，不直接调用积分模块；积分发放由 subscriber 异步完成。
	OrderPaidTopic = "order.paid"
	// OrderPaidSubscriber 是积分发放 subscriber 的稳定名称。
	// 它会写入 domain_event_deliveries，用于区分不同 subscriber 的处理状态。
	OrderPaidSubscriber = "points.award_on_order_paid"
	// OrderPaidPoints 是当前业务规则：订单支付成功奖励 10 积分。
	// 这个值会进入事件 payload，保证 subscriber 处理时看到的是发布时确定的规则快照。
	OrderPaidPoints = int64(10)
)

var (
	// ErrOrderPaidPointsSubscriberMissing 用于发布前保护。
	// PayOrder 会检查 order.paid 至少有积分 subscriber，避免订单已支付但积分逻辑未注册。
	ErrOrderPaidPointsSubscriberMissing = errors.New("order paid points subscriber is not registered")
	// ErrAwardOrderPaidPointsFuncMissing 表示启动注册时没有注入真正的积分发放用例。
	ErrAwardOrderPaidPointsFuncMissing = errors.New("award order paid points function is required")
	// ErrSendPointsRealtimeNotificationFuncMissing 表示启动注册时没有注入统一 notification 发送入口。
	ErrSendPointsRealtimeNotificationFuncMissing = errors.New("send points realtime notification function is required")

	// registerEventHandlersOnce 保证同一个进程内只注册一次 subscriber。
	// framework/events 要求 (topic, subscriber) 唯一；重复注册会导致启动失败。
	registerEventHandlersOnce sync.Once
	registerEventHandlersErr  error
)

// OrderPaidPayload 是 order.paid 事件的稳定 payload。
//
// 事件 payload 是业务 DTO，不是 models.Order。只放 subscriber 需要的稳定字段，
// 避免数据库模型字段变化时破坏事件契约。
type OrderPaidPayload struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Amount  int64  `json:"amount"`
	Points  int64  `json:"points"`
}

// AwardOrderPaidPointsCmd 是积分 subscriber 调用积分用例时使用的命令。
// 它从 OrderPaidPayload 派生，但保持独立，避免事件契约和积分用例参数互相绑死。
type AwardOrderPaidPointsCmd struct {
	UserID  string
	OrderID string
	Points  int64
}

// PointsResult 是积分发放后用于 realtime 通知的最小结果。
// subscriber 不需要知道完整积分流水，只需要知道哪个用户的余额变成多少。
type PointsResult struct {
	UserID  string
	Balance int64
}

// SendRealtimeNotificationCmd 是事件包注入 notification 封装时使用的最小命令。
// 这里不导入父级 api/usecase，避免 api/usecase/events 反向依赖具体 usecase 实现。
type SendRealtimeNotificationCmd struct {
	UserID       string
	MessageType  string
	Presentation string
	Payload      interface{}
}

// AwardOrderPaidPointsFunc 是从事件适配层注入的业务函数类型。
//
// api/usecase/events 负责订阅 order.paid，但真正的积分发放逻辑在 api/usecase。
// 用函数注入可以避免这个包反向依赖具体 usecase 实现，也方便测试替换。
//
// bool 返回值表示本次是否真的发放了积分；重复事件可能因为幂等约束不再发放。
type AwardOrderPaidPointsFunc func(fwusecase.Context, AwardOrderPaidPointsCmd) (PointsResult, bool, error)

// SendRealtimeNotificationFunc 是从启动层注入的统一 realtime notification 发送函数。
type SendRealtimeNotificationFunc func(fwusecase.Context, SendRealtimeNotificationCmd) error

// orderPaidPointsHandler 是 framework/events.Handler 的业务适配器。
// 它持有被注入的积分发放函数，并把事件 payload 转成积分命令。
type orderPaidPointsHandler struct {
	award AwardOrderPaidPointsFunc
	send  SendRealtimeNotificationFunc
}

// RegisterEventHandlers 注册 order.paid -> points.award_on_order_paid 订阅。
//
// 这个函数在应用启动时调用，不应该在 HTTP 请求过程中调用。注册后，PayOrder 发布
// order.paid 事件时，framework/events 会为该 subscriber 创建独立 delivery 和队列消息。
//
// RegisterTransactional 会让 Handle 在系统上下文中运行，并包一层 app DB 事务。
func RegisterEventHandlers(award AwardOrderPaidPointsFunc, send SendRealtimeNotificationFunc) error {
	if award == nil {
		return ErrAwardOrderPaidPointsFuncMissing
	}
	if send == nil {
		return ErrSendPointsRealtimeNotificationFuncMissing
	}

	registerEventHandlersOnce.Do(func() {
		registerEventHandlersErr = fwevents.RegisterTransactional[OrderPaidPayload](fwevents.Subscription{
			Topic:      OrderPaidTopic,
			Subscriber: OrderPaidSubscriber,
		}, orderPaidPointsHandler{award: award, send: send}.Handle)
	})
	return registerEventHandlersErr
}

// NewOrderPaidEvent 根据已支付订单构造 order.paid 领域事件。
//
// 发布方调用这个函数，只表达“订单已支付”的事实。积分、会员等后续动作由各自
// subscriber 监听同一个事件完成，发布方不需要知道有多少后续处理。
func NewOrderPaidEvent(order *models.Order) (fwevents.Event, error) {
	if order == nil {
		return fwevents.Event{}, fmt.Errorf("order is nil")
	}

	return fwevents.NewPayloadEvent(OrderPaidTopic, "order", order.ID, OrderPaidPayload{
		OrderID: order.ID,
		UserID:  order.UserID,
		Amount:  order.Amount,
		Points:  OrderPaidPoints,
	})
}

// Handle 处理单条 order.paid 事件投递。
//
// txCtx 来自 RegisterTransactional，已经是 SurfaceSystem 的 usecase context，
// 并且处于 app DB 事务中。积分写入成功后，realtime 通知通过 RegisterAfterCommit
// 注册到事务提交之后执行，避免“消息发出去了但积分事务回滚了”的不一致。
//
// 如果 award 返回 awarded=false，表示重复事件或幂等保护命中，不需要再次推送余额。
func (h orderPaidPointsHandler) Handle(txCtx fwusecase.Context, _ fwevents.Event, payload OrderPaidPayload) error {
	points, awarded, err := h.award(txCtx, AwardOrderPaidPointsCmd{
		UserID:  payload.UserID,
		OrderID: payload.OrderID,
		Points:  payload.Points,
	})
	if err != nil {
		return err
	}
	if !awarded {
		return nil
	}

	return fwusecase.RegisterAfterCommit(txCtx, func(runCtx context.Context) {
		notifyCtx := fwusecase.NewContext(runCtx, fwusecase.SurfaceSystem)
		_ = h.send(notifyCtx, SendRealtimeNotificationCmd{
			UserID:       points.UserID,
			MessageType:  "points",
			Presentation: "refresh",
			Payload: map[string]interface{}{
				"user_id": points.UserID,
				"balance": points.Balance,
			},
		})
	})
}
