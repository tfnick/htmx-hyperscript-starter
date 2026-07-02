package models

import (
	"context"
	"fmt"

	"github.com/tfnick/go-svelte-starter/api/db"
)

type OperationsAnalyticsQuery struct {
	StartAt        string `db:"start_at"`
	EndAt          string `db:"end_at"`
	EndBeforeStart string `db:"end_before_start"`
	OrganizationID string `db:"organization_id"`
	BucketFormat   string `db:"bucket_format"`
	Country        string `db:"country"`
	Limit          int    `db:"limit"`
}

type OperationsAnalyticsSummary struct {
	NewUsers                  int     `db:"new_users"`
	CumulativeUsers           int     `db:"cumulative_users"`
	ActiveAccountRatioPercent float64 `db:"active_account_ratio_percent"`
	NewPaidUsers              int     `db:"new_paid_users"`
	NewOrders                 int     `db:"new_orders"`
	PaidOrders                int     `db:"paid_orders"`
	PaidRevenueAmount         int64   `db:"paid_revenue_amount"`
	PaymentConversionPercent  float64 `db:"payment_conversion_percent"`
	AverageOrderValueAmount   int64   `db:"average_order_value_amount"`
	NewSubscriptions          int     `db:"new_subscriptions"`
	CanceledSubscriptions     int     `db:"canceled_subscriptions"`
	SubscriptionCancelPercent float64 `db:"subscription_cancel_percent"`
}

type OperationsAnalyticsTrendRow struct {
	Bucket                 string  `db:"bucket"`
	NewUsers               int     `db:"new_users"`
	CumulativeUsers        int     `db:"cumulative_users"`
	NewOrders              int     `db:"new_orders"`
	PaidOrders             int     `db:"paid_orders"`
	PaidRevenueAmount      int64   `db:"paid_revenue_amount"`
	NewSubscriptions       int     `db:"new_subscriptions"`
	CanceledSubscriptions  int     `db:"canceled_subscriptions"`
	SubscriptionCancelRate float64 `db:"subscription_cancel_rate"`
}

type OperationsAnalyticsGeoRow struct {
	Label   string  `db:"label"`
	Count   int     `db:"count"`
	Percent float64 `db:"percent"`
}

type OperationsAnalyticsHourRow struct {
	Hour  int `db:"hour"`
	Count int `db:"count"`
}

func OperationsAnalyticsSummaryByQuery(ctx context.Context, query OperationsAnalyticsQuery) (OperationsAnalyticsSummary, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return OperationsAnalyticsSummary{}, fmt.Errorf("database unavailable: %w", err)
	}

	var summary OperationsAnalyticsSummary
	err = eng.Get(&summary, `
		WITH
		scoped_users AS (
			SELECT u.*
			FROM users u
			WHERE 1=1
				#[ AND u.organization_id = :organization_id ]
		),
		period_users AS (
			SELECT *
			FROM scoped_users
			WHERE created_at >= :start_at AND created_at <= :end_at
		),
		scoped_orders AS (
			SELECT o.*, p.billing_type
			FROM orders o
			JOIN scoped_users u ON u.id = o.user_id
			LEFT JOIN products p ON p.id = o.product_id
		),
		period_orders AS (
			SELECT *
			FROM scoped_orders
			WHERE created_at >= :start_at AND created_at <= :end_at
		),
		first_paid_users AS (
			SELECT user_id, MIN(created_at) AS first_paid_at
			FROM scoped_orders
			WHERE status = 'paid'
			GROUP BY user_id
		),
		user_counts AS (
			SELECT
				COUNT(*) AS total_users,
				COALESCE(SUM(CASE WHEN is_active = 1 THEN 1 ELSE 0 END), 0) AS active_users
			FROM scoped_users
			WHERE created_at <= :end_at
		),
		order_counts AS (
			SELECT
				COUNT(*) AS total_orders,
				COALESCE(SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END), 0) AS paid_orders,
				COALESCE(SUM(CASE WHEN status = 'paid' THEN amount ELSE 0 END), 0) AS paid_revenue_amount,
				COALESCE(SUM(CASE WHEN status = 'paid' AND billing_type = 'subscription' THEN 1 ELSE 0 END), 0) AS new_subscriptions
			FROM period_orders
		),
		cancel_counts AS (
			SELECT COUNT(*) AS canceled_subscriptions
			FROM scoped_orders
			WHERE subscription_canceled_at >= :start_at AND subscription_canceled_at <= :end_at
		)
		SELECT
			(SELECT COUNT(*) FROM period_users) AS new_users,
			(SELECT total_users FROM user_counts) AS cumulative_users,
			CASE
				WHEN (SELECT total_users FROM user_counts) = 0 THEN 0
				ELSE CAST((SELECT active_users FROM user_counts) AS REAL) * 100.0 / (SELECT total_users FROM user_counts)
			END AS active_account_ratio_percent,
			(
				SELECT COUNT(*)
				FROM first_paid_users
				WHERE first_paid_at >= :start_at AND first_paid_at <= :end_at
			) AS new_paid_users,
			(SELECT total_orders FROM order_counts) AS new_orders,
			(SELECT paid_orders FROM order_counts) AS paid_orders,
			(SELECT paid_revenue_amount FROM order_counts) AS paid_revenue_amount,
			CASE
				WHEN (SELECT total_orders FROM order_counts) = 0 THEN 0
				ELSE CAST((SELECT paid_orders FROM order_counts) AS REAL) * 100.0 / (SELECT total_orders FROM order_counts)
			END AS payment_conversion_percent,
			CASE
				WHEN (SELECT paid_orders FROM order_counts) = 0 THEN 0
				ELSE CAST((SELECT paid_revenue_amount FROM order_counts) / (SELECT paid_orders FROM order_counts) AS INTEGER)
			END AS average_order_value_amount,
			(SELECT new_subscriptions FROM order_counts) AS new_subscriptions,
			(SELECT canceled_subscriptions FROM cancel_counts) AS canceled_subscriptions,
			CASE
				WHEN (SELECT new_subscriptions FROM order_counts) = 0 THEN 0
				ELSE CAST((SELECT canceled_subscriptions FROM cancel_counts) AS REAL) * 100.0 / (SELECT new_subscriptions FROM order_counts)
			END AS subscription_cancel_percent
	`, query)
	if err != nil {
		return OperationsAnalyticsSummary{}, fmt.Errorf("query operations analytics summary failed: %w", err)
	}
	return summary, nil
}

func OperationsAnalyticsTrendByQuery(ctx context.Context, query OperationsAnalyticsQuery) ([]OperationsAnalyticsTrendRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var rows []OperationsAnalyticsTrendRow
	err = eng.Select(&rows, `
		WITH
		scoped_users AS (
			SELECT u.*
			FROM users u
			WHERE 1=1
				#[ AND u.organization_id = :organization_id ]
		),
		user_buckets AS (
			SELECT strftime(:bucket_format, created_at) AS bucket, COUNT(*) AS new_users
			FROM scoped_users
			WHERE created_at >= :start_at AND created_at <= :end_at
			GROUP BY bucket
		),
		scoped_orders AS (
			SELECT o.*, p.billing_type
			FROM orders o
			JOIN scoped_users u ON u.id = o.user_id
			LEFT JOIN products p ON p.id = o.product_id
		),
		order_buckets AS (
			SELECT
				strftime(:bucket_format, created_at) AS bucket,
				COUNT(*) AS new_orders,
				COALESCE(SUM(CASE WHEN status = 'paid' THEN 1 ELSE 0 END), 0) AS paid_orders,
				COALESCE(SUM(CASE WHEN status = 'paid' THEN amount ELSE 0 END), 0) AS paid_revenue_amount,
				COALESCE(SUM(CASE WHEN status = 'paid' AND billing_type = 'subscription' THEN 1 ELSE 0 END), 0) AS new_subscriptions
			FROM scoped_orders
			WHERE created_at >= :start_at AND created_at <= :end_at
			GROUP BY bucket
		),
		cancel_buckets AS (
			SELECT strftime(:bucket_format, subscription_canceled_at) AS bucket, COUNT(*) AS canceled_subscriptions
			FROM scoped_orders
			WHERE subscription_canceled_at >= :start_at AND subscription_canceled_at <= :end_at
			GROUP BY bucket
		),
		all_buckets AS (
			SELECT bucket FROM user_buckets
			UNION
			SELECT bucket FROM order_buckets
			UNION
			SELECT bucket FROM cancel_buckets
		)
		SELECT
			b.bucket,
			COALESCE(ub.new_users, 0) AS new_users,
			(
				SELECT COUNT(*)
				FROM scoped_users u
				WHERE u.created_at < date(b.bucket, '+1 day')
			) AS cumulative_users,
			COALESCE(ob.new_orders, 0) AS new_orders,
			COALESCE(ob.paid_orders, 0) AS paid_orders,
			COALESCE(ob.paid_revenue_amount, 0) AS paid_revenue_amount,
			COALESCE(ob.new_subscriptions, 0) AS new_subscriptions,
			COALESCE(cb.canceled_subscriptions, 0) AS canceled_subscriptions,
			CASE
				WHEN COALESCE(ob.new_subscriptions, 0) = 0 THEN 0
				ELSE CAST(COALESCE(cb.canceled_subscriptions, 0) AS REAL) * 100.0 / COALESCE(ob.new_subscriptions, 0)
			END AS subscription_cancel_rate
		FROM all_buckets b
		LEFT JOIN user_buckets ub ON ub.bucket = b.bucket
		LEFT JOIN order_buckets ob ON ob.bucket = b.bucket
		LEFT JOIN cancel_buckets cb ON cb.bucket = b.bucket
		ORDER BY b.bucket ASC
	`, query)
	if err != nil {
		return nil, fmt.Errorf("query operations analytics trend failed: %w", err)
	}
	return rows, nil
}

func OperationsAnalyticsCountriesByQuery(ctx context.Context, query OperationsAnalyticsQuery) ([]OperationsAnalyticsGeoRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var rows []OperationsAnalyticsGeoRow
	err = eng.Select(&rows, `
		SELECT
			CASE
				WHEN TRIM(COALESCE(urp.registration_country, '')) = '' THEN 'Unknown'
				ELSE TRIM(COALESCE(urp.registration_country, ''))
			END AS label,
			COUNT(u.id) AS count,
			CASE
				WHEN COUNT(*) OVER () = 0 THEN 0
				ELSE CAST(COUNT(u.id) AS REAL) * 100.0 / SUM(COUNT(u.id)) OVER ()
			END AS percent
		FROM users u
		LEFT JOIN user_registration_profiles urp ON urp.user_id = u.id
		WHERE u.created_at >= :start_at AND u.created_at <= :end_at
			#[ AND u.organization_id = :organization_id ]
		GROUP BY label
		ORDER BY count DESC, label ASC
		LIMIT :limit
	`, query)
	if err != nil {
		return nil, fmt.Errorf("query operations analytics countries failed: %w", err)
	}
	return rows, nil
}

func OperationsAnalyticsRegionsByQuery(ctx context.Context, query OperationsAnalyticsQuery) ([]OperationsAnalyticsGeoRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var rows []OperationsAnalyticsGeoRow
	err = eng.Select(&rows, `
		SELECT
			CASE
				WHEN TRIM(COALESCE(urp.registration_region, '')) = '' THEN 'Unknown'
				ELSE TRIM(COALESCE(urp.registration_region, ''))
			END AS label,
			COUNT(u.id) AS count,
			CASE
				WHEN COUNT(*) OVER () = 0 THEN 0
				ELSE CAST(COUNT(u.id) AS REAL) * 100.0 / SUM(COUNT(u.id)) OVER ()
			END AS percent
		FROM users u
		LEFT JOIN user_registration_profiles urp ON urp.user_id = u.id
		WHERE u.created_at >= :start_at AND u.created_at <= :end_at
			#[ AND u.organization_id = :organization_id ]
			#[ AND ((:country = 'Unknown' AND TRIM(COALESCE(urp.registration_country, '')) = '') OR TRIM(urp.registration_country) = :country) ]
		GROUP BY label
		ORDER BY count DESC, label ASC
		LIMIT :limit
	`, query)
	if err != nil {
		return nil, fmt.Errorf("query operations analytics regions failed: %w", err)
	}
	return rows, nil
}

func OperationsAnalyticsSourceDistributionByQuery(ctx context.Context, query OperationsAnalyticsQuery) ([]OperationsAnalyticsGeoRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var rows []OperationsAnalyticsGeoRow
	err = eng.Select(&rows, `
		SELECT
			TRIM(COALESCE(urp.utm_source, '')) AS label,
			COUNT(u.id) AS count,
			CASE
				WHEN COUNT(*) OVER () = 0 THEN 0
				ELSE CAST(COUNT(u.id) AS REAL) * 100.0 / SUM(COUNT(u.id)) OVER ()
			END AS percent
		FROM users u
		JOIN user_registration_profiles urp ON urp.user_id = u.id
		WHERE u.created_at >= :start_at AND u.created_at <= :end_at
			#[ AND u.organization_id = :organization_id ]
			AND TRIM(COALESCE(urp.utm_source, '')) != ''
		GROUP BY label
		ORDER BY count DESC, label ASC
		LIMIT :limit
	`, query)
	if err != nil {
		return nil, fmt.Errorf("query operations analytics source distribution failed: %w", err)
	}
	return rows, nil
}

func OperationsAnalyticsRegistrationHoursByQuery(ctx context.Context, query OperationsAnalyticsQuery) ([]OperationsAnalyticsHourRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var rows []OperationsAnalyticsHourRow
	err = eng.Select(&rows, `
		WITH RECURSIVE hours(hour) AS (
			SELECT 0
			UNION ALL
			SELECT hour + 1 FROM hours WHERE hour < 23
		),
		user_hours AS (
			SELECT CAST(strftime('%H', created_at) AS INTEGER) AS hour, COUNT(*) AS count
			FROM users u
			WHERE u.created_at >= :start_at AND u.created_at <= :end_at
				#[ AND u.organization_id = :organization_id ]
			GROUP BY hour
		)
		SELECT
			h.hour,
			COALESCE(uh.count, 0) AS count
		FROM hours h
		LEFT JOIN user_hours uh ON uh.hour = h.hour
		ORDER BY h.hour ASC
	`, query)
	if err != nil {
		return nil, fmt.Errorf("query operations analytics registration hours failed: %w", err)
	}
	return rows, nil
}
