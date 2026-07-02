package usecase

import (
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

var operationsAnalyticsNowUTC = timefmt.NowUTC

type OperationsAnalyticsQry struct {
	Period  string
	Month   string
	Country string
}

type OperationsAnalyticsCo struct {
	Period              OperationsAnalyticsPeriodCo
	Summary             OperationsAnalyticsSummaryCo
	Trends              OperationsAnalyticsTrendsCo
	Geo                 OperationsAnalyticsGeoCo
	RegistrationHours   []OperationsAnalyticsHourCo
	SourceDistribution  []OperationsAnalyticsGeoItemCo
}

type OperationsAnalyticsPeriodCo struct {
	Key     string
	Label   string
	StartAt string
	EndAt   string
}

type OperationsAnalyticsMetricCo struct {
	Value      float64
	MoMPercent *float64
	YoYPercent *float64
}

type OperationsAnalyticsSummaryCo struct {
	NewUsers                  OperationsAnalyticsMetricCo
	CumulativeUsers           OperationsAnalyticsMetricCo
	ActiveAccountRatioPercent OperationsAnalyticsMetricCo
	NewPaidUsers              OperationsAnalyticsMetricCo
	NewOrders                 OperationsAnalyticsMetricCo
	PaidOrders                OperationsAnalyticsMetricCo
	PaidRevenueAmount         OperationsAnalyticsMetricCo
	PaymentConversionPercent  OperationsAnalyticsMetricCo
	AverageOrderValueAmount   OperationsAnalyticsMetricCo
	NewSubscriptions          OperationsAnalyticsMetricCo
	CanceledSubscriptions     OperationsAnalyticsMetricCo
	SubscriptionCancelPercent OperationsAnalyticsMetricCo
}

type OperationsAnalyticsTrendsCo struct {
	UserGrowth   []OperationsAnalyticsTrendPointCo
	OrderGrowth  []OperationsAnalyticsTrendPointCo
	Subscription []OperationsAnalyticsTrendPointCo
}

type OperationsAnalyticsTrendPointCo struct {
	Bucket                 string
	NewUsers               int
	CumulativeUsers        int
	NewOrders              int
	PaidOrders             int
	PaidRevenueAmount      int64
	NewSubscriptions       int
	CanceledSubscriptions  int
	SubscriptionCancelRate float64
}

type OperationsAnalyticsGeoCo struct {
	SelectedCountry string
	Countries       []OperationsAnalyticsGeoItemCo
	Regions         []OperationsAnalyticsGeoItemCo
}

type OperationsAnalyticsGeoItemCo struct {
	Label   string
	Count   int
	Percent float64
}

type OperationsAnalyticsHourCo struct {
	Hour  int
	Count int
}

func GetOperationsAnalytics(ctx fwusecase.Context, qry OperationsAnalyticsQry) (OperationsAnalyticsCo, error) {
	period, err := normalizeAnalyticsPeriod(qry.Period, qry.Month, operationsAnalyticsNowUTC())
	if err != nil {
		return OperationsAnalyticsCo{}, err
	}
	scope, err := operationsAnalyticsScope(ctx)
	if err != nil {
		return OperationsAnalyticsCo{}, err
	}

	baseQuery := models.OperationsAnalyticsQuery{
		StartAt:        timefmt.SQLiteDateTime(period.CurrentStart),
		EndAt:          timefmt.SQLiteDateTime(period.CurrentEnd),
		EndBeforeStart: timefmt.SQLiteDateTime(period.CurrentStart.Add(-time.Second)),
		OrganizationID: scope.OrganizationID,
		BucketFormat:   operationsAnalyticsBucketFormat(period.Key),
		Limit:          8,
	}

	current, err := models.OperationsAnalyticsSummaryByQuery(ctx.Std(), baseQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics summary", err)
	}
	previousQuery := baseQuery
	previousQuery.StartAt = timefmt.SQLiteDateTime(period.PreviousStart)
	previousQuery.EndAt = timefmt.SQLiteDateTime(period.PreviousEnd)
	previous, err := models.OperationsAnalyticsSummaryByQuery(ctx.Std(), previousQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics comparison", err)
	}
	yoyQuery := baseQuery
	yoyQuery.StartAt = timefmt.SQLiteDateTime(period.YoYStart)
	yoyQuery.EndAt = timefmt.SQLiteDateTime(period.YoYEnd)
	yoy, err := models.OperationsAnalyticsSummaryByQuery(ctx.Std(), yoyQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics comparison", err)
	}

	trendRows, err := models.OperationsAnalyticsTrendByQuery(ctx.Std(), baseQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics trends", err)
	}

	countries, err := models.OperationsAnalyticsCountriesByQuery(ctx.Std(), baseQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics geo distribution", err)
	}
	selectedCountry := strings.TrimSpace(qry.Country)
	regionQuery := baseQuery
	if selectedCountry != "" {
		regionQuery.Country = selectedCountry
	}
	regions, err := models.OperationsAnalyticsRegionsByQuery(ctx.Std(), regionQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics region distribution", err)
	}

	hours, err := models.OperationsAnalyticsRegistrationHoursByQuery(ctx.Std(), baseQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics registration hours", err)
	}

	sources, err := models.OperationsAnalyticsSourceDistributionByQuery(ctx.Std(), baseQuery)
	if err != nil {
		return OperationsAnalyticsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load operations analytics source distribution", err)
	}

	trends := operationsAnalyticsTrendsFromRows(trendRows)
	return OperationsAnalyticsCo{
		Period: OperationsAnalyticsPeriodCo{
			Key:     period.Key,
			Label:   period.Label,
			StartAt: timefmt.RFC3339(period.CurrentStart),
			EndAt:   timefmt.RFC3339(period.CurrentEnd),
		},
		Summary:           operationsAnalyticsSummaryFromModels(current, previous, yoy),
		Trends:            trends,
		Geo:               operationsAnalyticsGeoFromModels(selectedCountry, countries, regions),
		RegistrationHours: operationsAnalyticsHoursFromModels(hours),
		SourceDistribution: operationsAnalyticsGeoItems(sources),
	}, nil
}

type operationsAnalyticsScopeCo struct {
	OrganizationID string
}

func operationsAnalyticsScope(ctx fwusecase.Context) (operationsAnalyticsScopeCo, error) {
	if !ctx.Actor.Authenticated {
		return operationsAnalyticsScopeCo{}, fwusecase.E(fwusecase.CodeUnauthorized, "not logged in", nil)
	}
	if ctx.HasActorPermission(authz.PermissionOrdersReadAll) && ctx.HasActorPermission(authz.PermissionUsersReadAll) {
		return operationsAnalyticsScopeCo{}, nil
	}
	if ctx.HasActorPermission(authz.PermissionOrdersReadOrg) && ctx.HasActorPermission(authz.PermissionUsersReadOrg) {
		orgID := strings.TrimSpace(ctx.Actor.OrganizationID)
		if orgID == "" {
			return operationsAnalyticsScopeCo{}, fwusecase.E(fwusecase.CodeForbidden, "organization scope is required", nil)
		}
		return operationsAnalyticsScopeCo{OrganizationID: orgID}, nil
	}
	return operationsAnalyticsScopeCo{}, fwusecase.E(fwusecase.CodeForbidden, "permission is required", nil)
}

func operationsAnalyticsBucketFormat(periodKey string) string {
	return "%Y-%m-%d"
}

func operationsAnalyticsSummaryFromModels(current models.OperationsAnalyticsSummary, previous models.OperationsAnalyticsSummary, yoy models.OperationsAnalyticsSummary) OperationsAnalyticsSummaryCo {
	return OperationsAnalyticsSummaryCo{
		NewUsers:                  analyticsMetric(float64(current.NewUsers), float64(previous.NewUsers), float64(yoy.NewUsers)),
		CumulativeUsers:           analyticsMetric(float64(current.CumulativeUsers), float64(previous.CumulativeUsers), float64(yoy.CumulativeUsers)),
		ActiveAccountRatioPercent: analyticsMetric(current.ActiveAccountRatioPercent, previous.ActiveAccountRatioPercent, yoy.ActiveAccountRatioPercent),
		NewPaidUsers:              analyticsMetric(float64(current.NewPaidUsers), float64(previous.NewPaidUsers), float64(yoy.NewPaidUsers)),
		NewOrders:                 analyticsMetric(float64(current.NewOrders), float64(previous.NewOrders), float64(yoy.NewOrders)),
		PaidOrders:                analyticsMetric(float64(current.PaidOrders), float64(previous.PaidOrders), float64(yoy.PaidOrders)),
		PaidRevenueAmount:         analyticsMetric(float64(current.PaidRevenueAmount), float64(previous.PaidRevenueAmount), float64(yoy.PaidRevenueAmount)),
		PaymentConversionPercent:  analyticsMetric(current.PaymentConversionPercent, previous.PaymentConversionPercent, yoy.PaymentConversionPercent),
		AverageOrderValueAmount:   analyticsMetric(float64(current.AverageOrderValueAmount), float64(previous.AverageOrderValueAmount), float64(yoy.AverageOrderValueAmount)),
		NewSubscriptions:          analyticsMetric(float64(current.NewSubscriptions), float64(previous.NewSubscriptions), float64(yoy.NewSubscriptions)),
		CanceledSubscriptions:     analyticsMetric(float64(current.CanceledSubscriptions), float64(previous.CanceledSubscriptions), float64(yoy.CanceledSubscriptions)),
		SubscriptionCancelPercent: analyticsMetric(current.SubscriptionCancelPercent, previous.SubscriptionCancelPercent, yoy.SubscriptionCancelPercent),
	}
}

func analyticsMetric(current float64, previous float64, yoy float64) OperationsAnalyticsMetricCo {
	return OperationsAnalyticsMetricCo{
		Value:      current,
		MoMPercent: percentChangeFloat(current, previous),
		YoYPercent: percentChangeFloat(current, yoy),
	}
}

func operationsAnalyticsTrendsFromRows(rows []models.OperationsAnalyticsTrendRow) OperationsAnalyticsTrendsCo {
	userGrowth := make([]OperationsAnalyticsTrendPointCo, 0, len(rows))
	orderGrowth := make([]OperationsAnalyticsTrendPointCo, 0, len(rows))
	subscription := make([]OperationsAnalyticsTrendPointCo, 0, len(rows))
	for i := range rows {
		point := OperationsAnalyticsTrendPointCo{
			Bucket:                 rows[i].Bucket,
			NewUsers:               rows[i].NewUsers,
			CumulativeUsers:        rows[i].CumulativeUsers,
			NewOrders:              rows[i].NewOrders,
			PaidOrders:             rows[i].PaidOrders,
			PaidRevenueAmount:      rows[i].PaidRevenueAmount,
			NewSubscriptions:       rows[i].NewSubscriptions,
			CanceledSubscriptions:  rows[i].CanceledSubscriptions,
			SubscriptionCancelRate: rows[i].SubscriptionCancelRate,
		}
		userGrowth = append(userGrowth, point)
		orderGrowth = append(orderGrowth, point)
		subscription = append(subscription, point)
	}
	return OperationsAnalyticsTrendsCo{
		UserGrowth:   userGrowth,
		OrderGrowth:  orderGrowth,
		Subscription: subscription,
	}
}

func operationsAnalyticsGeoFromModels(selectedCountry string, countries []models.OperationsAnalyticsGeoRow, regions []models.OperationsAnalyticsGeoRow) OperationsAnalyticsGeoCo {
	return OperationsAnalyticsGeoCo{
		SelectedCountry: selectedCountry,
		Countries:       operationsAnalyticsGeoItems(countries),
		Regions:         operationsAnalyticsGeoItems(regions),
	}
}

func operationsAnalyticsGeoItems(rows []models.OperationsAnalyticsGeoRow) []OperationsAnalyticsGeoItemCo {
	items := make([]OperationsAnalyticsGeoItemCo, 0, len(rows))
	for i := range rows {
		items = append(items, OperationsAnalyticsGeoItemCo{
			Label:   rows[i].Label,
			Count:   rows[i].Count,
			Percent: rows[i].Percent,
		})
	}
	return items
}

func NormalizeAnalyticsPeriod(ctx fwusecase.Context, period, month string) (time.Time, time.Time, error) {
	analyticsPeriod, err := normalizeAnalyticsPeriod(period, month, operationsAnalyticsNowUTC())
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return analyticsPeriod.CurrentStart, analyticsPeriod.CurrentEnd, nil
}

type FunnelCo struct {
	Visitors   int     `json:"visitors"`
	Registered int     `json:"registered"`
	Ordered    int     `json:"ordered"`
	Paid       int     `json:"paid"`
	RegRate    float64 `json:"reg_rate"`
	OrderRate  float64 `json:"order_rate"`
	PayRate    float64 `json:"pay_rate"`
}

type PVUVCo struct {
	Items []PVUVItem `json:"items"`
}

type PVUVItem struct {
	Date    string `json:"date"`
	Country string `json:"country"`
	PV      int    `json:"pv"`
	UV      int    `json:"uv"`
}

func GetFunnelAnalytics(ctx fwusecase.Context, startAt, endAt time.Time) (FunnelCo, error) {
	rows, err := models.FunnelCounts(ctx.Std(), startAt, endAt)
	if err != nil {
		return FunnelCo{}, err
	}

	m := map[string]int{}
	for _, r := range rows {
		m[r.Stage] = r.Count
	}

	co := FunnelCo{
		Visitors:   m["visitors"],
		Registered: m["registered"],
		Ordered:    m["ordered"],
		Paid:       m["paid"],
	}

	if co.Visitors > 0 {
		co.RegRate = float64(co.Registered) / float64(co.Visitors) * 100
	}
	if co.Registered > 0 {
		co.OrderRate = float64(co.Ordered) / float64(co.Registered) * 100
	}
	if co.Ordered > 0 {
		co.PayRate = float64(co.Paid) / float64(co.Ordered) * 100
	}

	return co, nil
}

func GetPVUVData(ctx fwusecase.Context, startAt, endAt time.Time) (PVUVCo, error) {
	rows, err := models.PVUVByDateAndCountry(ctx.Std(), startAt, endAt)
	if err != nil {
		return PVUVCo{}, err
	}
	items := make([]PVUVItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, PVUVItem{
			Date:    r.Date,
			Country: r.Country,
			PV:      r.PV,
			UV:      r.UV,
		})
	}
	return PVUVCo{Items: items}, nil
}

func operationsAnalyticsHoursFromModels(rows []models.OperationsAnalyticsHourRow) []OperationsAnalyticsHourCo {
	items := make([]OperationsAnalyticsHourCo, 0, len(rows))
	for i := range rows {
		items = append(items, OperationsAnalyticsHourCo{
			Hour:  rows[i].Hour,
			Count: rows[i].Count,
		})
	}
	return items
}
