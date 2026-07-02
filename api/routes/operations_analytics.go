package routes

import (
	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type OperationsAnalyticsResponse struct {
	Period             OperationsAnalyticsPeriodResponse          `json:"period"`
	Summary            OperationsAnalyticsSummaryResponse         `json:"summary"`
	Trends             OperationsAnalyticsTrendsResponse          `json:"trends"`
	Geo                OperationsAnalyticsGeoResponse             `json:"geo"`
	RegistrationHours  []OperationsAnalyticsHourResponse          `json:"registration_hours"`
	SourceDistribution []OperationsAnalyticsGeoItemResponse       `json:"source_distribution"`
}

type OperationsAnalyticsPeriodResponse struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
}

type OperationsAnalyticsMetricResponse struct {
	Value      float64  `json:"value"`
	MoMPercent *float64 `json:"mom_percent"`
	YoYPercent *float64 `json:"yoy_percent"`
}

type OperationsAnalyticsSummaryResponse struct {
	NewUsers                  OperationsAnalyticsMetricResponse `json:"new_users"`
	CumulativeUsers           OperationsAnalyticsMetricResponse `json:"cumulative_users"`
	ActiveAccountRatioPercent OperationsAnalyticsMetricResponse `json:"active_account_ratio_percent"`
	NewPaidUsers              OperationsAnalyticsMetricResponse `json:"new_paid_users"`
	NewOrders                 OperationsAnalyticsMetricResponse `json:"new_orders"`
	PaidOrders                OperationsAnalyticsMetricResponse `json:"paid_orders"`
	PaidRevenueAmount         OperationsAnalyticsMetricResponse `json:"paid_revenue_amount"`
	PaymentConversionPercent  OperationsAnalyticsMetricResponse `json:"payment_conversion_percent"`
	AverageOrderValueAmount   OperationsAnalyticsMetricResponse `json:"average_order_value_amount"`
	NewSubscriptions          OperationsAnalyticsMetricResponse `json:"new_subscriptions"`
	CanceledSubscriptions     OperationsAnalyticsMetricResponse `json:"canceled_subscriptions"`
	SubscriptionCancelPercent OperationsAnalyticsMetricResponse `json:"subscription_cancel_percent"`
}

type OperationsAnalyticsTrendsResponse struct {
	UserGrowth   []OperationsAnalyticsTrendPointResponse `json:"user_growth"`
	OrderGrowth  []OperationsAnalyticsTrendPointResponse `json:"order_growth"`
	Subscription []OperationsAnalyticsTrendPointResponse `json:"subscription"`
}

type OperationsAnalyticsTrendPointResponse struct {
	Bucket                 string  `json:"bucket"`
	NewUsers               int     `json:"new_users"`
	CumulativeUsers        int     `json:"cumulative_users"`
	NewOrders              int     `json:"new_orders"`
	PaidOrders             int     `json:"paid_orders"`
	PaidRevenueAmount      int64   `json:"paid_revenue_amount"`
	NewSubscriptions       int     `json:"new_subscriptions"`
	CanceledSubscriptions  int     `json:"canceled_subscriptions"`
	SubscriptionCancelRate float64 `json:"subscription_cancel_rate"`
}

type OperationsAnalyticsGeoResponse struct {
	SelectedCountry string                               `json:"selected_country"`
	Countries       []OperationsAnalyticsGeoItemResponse `json:"countries"`
	Regions         []OperationsAnalyticsGeoItemResponse `json:"regions"`
}

type OperationsAnalyticsGeoItemResponse struct {
	Label   string  `json:"label"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type OperationsAnalyticsHourResponse struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

func GetOperationsAnalytics(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	analytics, err := usecase.GetOperationsAnalytics(ctx, usecase.OperationsAnalyticsQry{
		Period:  c.QueryParam("period"),
		Month:   c.QueryParam("month"),
		Country: c.QueryParam("country"),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToOperationsAnalyticsResponse(analytics))
}

func ToOperationsAnalyticsResponse(analytics usecase.OperationsAnalyticsCo) OperationsAnalyticsResponse {
	return OperationsAnalyticsResponse{
		Period: OperationsAnalyticsPeriodResponse{
			Key:     analytics.Period.Key,
			Label:   analytics.Period.Label,
			StartAt: analytics.Period.StartAt,
			EndAt:   analytics.Period.EndAt,
		},
		Summary:            toOperationsAnalyticsSummaryResponse(analytics.Summary),
		Trends:             toOperationsAnalyticsTrendsResponse(analytics.Trends),
		Geo:                toOperationsAnalyticsGeoResponse(analytics.Geo),
		RegistrationHours:  toOperationsAnalyticsHourResponses(analytics.RegistrationHours),
		SourceDistribution: toOperationsAnalyticsGeoItemResponses(analytics.SourceDistribution),
	}
}

func toOperationsAnalyticsSummaryResponse(summary usecase.OperationsAnalyticsSummaryCo) OperationsAnalyticsSummaryResponse {
	return OperationsAnalyticsSummaryResponse{
		NewUsers:                  toOperationsAnalyticsMetricResponse(summary.NewUsers),
		CumulativeUsers:           toOperationsAnalyticsMetricResponse(summary.CumulativeUsers),
		ActiveAccountRatioPercent: toOperationsAnalyticsMetricResponse(summary.ActiveAccountRatioPercent),
		NewPaidUsers:              toOperationsAnalyticsMetricResponse(summary.NewPaidUsers),
		NewOrders:                 toOperationsAnalyticsMetricResponse(summary.NewOrders),
		PaidOrders:                toOperationsAnalyticsMetricResponse(summary.PaidOrders),
		PaidRevenueAmount:         toOperationsAnalyticsMetricResponse(summary.PaidRevenueAmount),
		PaymentConversionPercent:  toOperationsAnalyticsMetricResponse(summary.PaymentConversionPercent),
		AverageOrderValueAmount:   toOperationsAnalyticsMetricResponse(summary.AverageOrderValueAmount),
		NewSubscriptions:          toOperationsAnalyticsMetricResponse(summary.NewSubscriptions),
		CanceledSubscriptions:     toOperationsAnalyticsMetricResponse(summary.CanceledSubscriptions),
		SubscriptionCancelPercent: toOperationsAnalyticsMetricResponse(summary.SubscriptionCancelPercent),
	}
}

func toOperationsAnalyticsMetricResponse(metric usecase.OperationsAnalyticsMetricCo) OperationsAnalyticsMetricResponse {
	return OperationsAnalyticsMetricResponse{
		Value:      metric.Value,
		MoMPercent: metric.MoMPercent,
		YoYPercent: metric.YoYPercent,
	}
}

func toOperationsAnalyticsTrendsResponse(trends usecase.OperationsAnalyticsTrendsCo) OperationsAnalyticsTrendsResponse {
	return OperationsAnalyticsTrendsResponse{
		UserGrowth:   toOperationsAnalyticsTrendPointResponses(trends.UserGrowth),
		OrderGrowth:  toOperationsAnalyticsTrendPointResponses(trends.OrderGrowth),
		Subscription: toOperationsAnalyticsTrendPointResponses(trends.Subscription),
	}
}

func toOperationsAnalyticsTrendPointResponses(points []usecase.OperationsAnalyticsTrendPointCo) []OperationsAnalyticsTrendPointResponse {
	responses := make([]OperationsAnalyticsTrendPointResponse, 0, len(points))
	for i := range points {
		responses = append(responses, OperationsAnalyticsTrendPointResponse{
			Bucket:                 points[i].Bucket,
			NewUsers:               points[i].NewUsers,
			CumulativeUsers:        points[i].CumulativeUsers,
			NewOrders:              points[i].NewOrders,
			PaidOrders:             points[i].PaidOrders,
			PaidRevenueAmount:      points[i].PaidRevenueAmount,
			NewSubscriptions:       points[i].NewSubscriptions,
			CanceledSubscriptions:  points[i].CanceledSubscriptions,
			SubscriptionCancelRate: points[i].SubscriptionCancelRate,
		})
	}
	return responses
}

func toOperationsAnalyticsGeoResponse(geo usecase.OperationsAnalyticsGeoCo) OperationsAnalyticsGeoResponse {
	return OperationsAnalyticsGeoResponse{
		SelectedCountry: geo.SelectedCountry,
		Countries:       toOperationsAnalyticsGeoItemResponses(geo.Countries),
		Regions:         toOperationsAnalyticsGeoItemResponses(geo.Regions),
	}
}

func toOperationsAnalyticsGeoItemResponses(items []usecase.OperationsAnalyticsGeoItemCo) []OperationsAnalyticsGeoItemResponse {
	responses := make([]OperationsAnalyticsGeoItemResponse, 0, len(items))
	for i := range items {
		responses = append(responses, OperationsAnalyticsGeoItemResponse{
			Label:   items[i].Label,
			Count:   items[i].Count,
			Percent: items[i].Percent,
		})
	}
	return responses
}

func toOperationsAnalyticsHourResponses(items []usecase.OperationsAnalyticsHourCo) []OperationsAnalyticsHourResponse {
	responses := make([]OperationsAnalyticsHourResponse, 0, len(items))
	for i := range items {
		responses = append(responses, OperationsAnalyticsHourResponse{
			Hour:  items[i].Hour,
			Count: items[i].Count,
		})
	}
	return responses
}

type FunnelResponse struct {
	Visitors   int     `json:"visitors"`
	Registered int     `json:"registered"`
	Ordered    int     `json:"ordered"`
	Paid       int     `json:"paid"`
	RegRate    float64 `json:"reg_rate"`
	OrderRate  float64 `json:"order_rate"`
	PayRate    float64 `json:"pay_rate"`
}

type PVUVResponse struct {
	Items []PVUVItemResponse `json:"items"`
}

type PVUVItemResponse struct {
	Date    string `json:"date"`
	Country string `json:"country"`
	PV      int    `json:"pv"`
	UV      int    `json:"uv"`
}

func GetFunnelAnalytics(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	period, month := analyticsPeriodFromRequest(c)
	startAt, endAt, err := usecase.NormalizeAnalyticsPeriod(ctx, period, month)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	funnel, err := usecase.GetFunnelAnalytics(ctx, startAt, endAt)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, FunnelResponse{
		Visitors:   funnel.Visitors,
		Registered: funnel.Registered,
		Ordered:    funnel.Ordered,
		Paid:       funnel.Paid,
		RegRate:    funnel.RegRate,
		OrderRate:  funnel.OrderRate,
		PayRate:    funnel.PayRate,
	})
}

func GetPVUVData(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	period, month := analyticsPeriodFromRequest(c)
	startAt, endAt, err := usecase.NormalizeAnalyticsPeriod(ctx, period, month)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	data, err := usecase.GetPVUVData(ctx, startAt, endAt)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	items := make([]PVUVItemResponse, 0, len(data.Items))
	for _, item := range data.Items {
		items = append(items, PVUVItemResponse{
			Date:    item.Date,
			Country: item.Country,
			PV:      item.PV,
			UV:      item.UV,
		})
	}
	return httpresponse.OK(c, PVUVResponse{Items: items})
}

func analyticsPeriodFromRequest(c echo.Context) (string, string) {
	return c.QueryParam("period"), c.QueryParam("month")
}
