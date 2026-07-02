package events

import (
	"errors"
	"sync"

	fwevents "github.com/tfnick/go-svelte-starter/api/framework/events"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

// OrderPaidMembershipSubscriber 是会员开通 subscriber 的稳定名称。
//
// 它和积分 subscriber 监听同一个 order.paid 事件，但拥有独立的 delivery row
// 和 queue message。积分失败不会阻塞会员处理，会员失败也不会重跑积分处理。
const OrderPaidMembershipSubscriber = "membership.apply_on_order_paid"

var (
	// ErrApplyOrderMembershipFuncMissing 表示启动注册时没有注入会员开通用例。
	// 没有业务函数时注册 subscriber 没有意义，因此启动阶段应直接失败。
	ErrApplyOrderMembershipFuncMissing = errors.New("apply order membership function is required")

	// registerMembershipHandlersOnce 保证进程内只注册一次会员 subscriber。
	// framework/events 要求同一个 topic + subscriber 名称唯一。
	registerMembershipHandlersOnce sync.Once
	registerMembershipHandlersErr  error
)

// ApplyOrderMembershipCmd 是会员 subscriber 调用业务用例时使用的命令。
//
// order.paid payload 里有 UserID、Amount、Points 等信息，但会员开通只需要订单 ID。
// 具体会员等级、有效期、是否订阅商品等规则由业务用例根据订单和商品配置重新读取。
type ApplyOrderMembershipCmd struct {
	OrderID string
}

// MembershipResult 是会员开通后的最小业务结果。
// 当前 subscriber 不直接使用它，但保留这个结果可以让调用方在测试或未来通知场景中复用。
type MembershipResult struct {
	UserID              string
	MembershipLevel     string
	MembershipExpiresAt string
}

// ApplyOrderMembershipFunc 是从事件适配层注入的会员开通业务函数。
//
// 用函数类型注入可以让 api/usecase/events 只负责事件订阅适配，不直接耦合具体
// usecase 实现。bool 返回值表示本次是否真的应用了会员权益；重复事件可能因为
// 幂等保护返回 false。
type ApplyOrderMembershipFunc func(fwusecase.Context, ApplyOrderMembershipCmd) (MembershipResult, bool, error)

// orderPaidMembershipHandler 是 framework/events.Handler 的业务适配器。
// 它把 order.paid payload 转成 ApplyOrderMembershipCmd，并调用被注入的业务函数。
type orderPaidMembershipHandler struct {
	apply ApplyOrderMembershipFunc
}

// RegisterMembershipEventHandlers 注册 order.paid -> membership.apply_on_order_paid 订阅。
//
// 这个函数在应用启动时调用，不应该在请求路径中重复注册。RegisterTransactional
// 会在处理每条消息时创建 SurfaceSystem 的 usecase context，并在 app DB 事务中执行 Handle。
func RegisterMembershipEventHandlers(apply ApplyOrderMembershipFunc) error {
	if apply == nil {
		return ErrApplyOrderMembershipFuncMissing
	}

	registerMembershipHandlersOnce.Do(func() {
		registerMembershipHandlersErr = fwevents.RegisterTransactional[OrderPaidPayload](fwevents.Subscription{
			Topic:      OrderPaidTopic,
			Subscriber: OrderPaidMembershipSubscriber,
		}, orderPaidMembershipHandler{apply: apply}.Handle)
	})
	return registerMembershipHandlersErr
}

// Handle 处理单条 order.paid 事件投递。
//
// 会员开通逻辑只需要订单 ID：业务用例会加载订单、商品和当前用户会员状态，
// 决定是否应用权益。这里忽略 MembershipResult 和 applied，是因为当前 subscriber
// 只负责把权益落库；没有额外 realtime 通知或后续动作需要这些返回值。
//
// 如果 apply 返回 error，framework/events 会把该 subscriber 的 delivery 标记为 failed，
// 队列消息保持可重试。因此 ApplyOrderMembershipFunc 本身必须是幂等的。
func (h orderPaidMembershipHandler) Handle(txCtx fwusecase.Context, _ fwevents.Event, payload OrderPaidPayload) error {
	_, _, err := h.apply(txCtx, ApplyOrderMembershipCmd{
		OrderID: payload.OrderID,
	})
	return err
}
