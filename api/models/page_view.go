package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
)

type PageView struct {
	ID          string `db:"id"`
	UserID      string `db:"user_id"`
	SessionID   string `db:"session_id"`
	Path        string `db:"path"`
	Country     string `db:"country"`
	Region      string `db:"region"`
	Referrer    string `db:"referrer"`
	UtmSource   string `db:"utm_source"`
	UtmMedium   string `db:"utm_medium"`
	UtmCampaign string `db:"utm_campaign"`
	CreatedAt   string `db:"created_at"`
}

func InsertPageView(ctx context.Context, pv *PageView) error {
	pv.ID = uuid.Must(uuid.NewV7()).String()
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return fmt.Errorf("get engine for insert page view: %w", err)
	}
	_, err = d.ExecP(
		`INSERT INTO page_views (id, user_id, session_id, path, country, region, referrer, utm_source, utm_medium, utm_campaign, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pv.ID, pv.UserID, pv.SessionID, pv.Path, pv.Country, pv.Region, pv.Referrer,
		pv.UtmSource, pv.UtmMedium, pv.UtmCampaign, timefmt.NowSQLiteDateTime(),
	)
	if err != nil {
		return fmt.Errorf("insert page view: %w", err)
	}
	return nil
}

type PVUVRow struct {
	Date    string `db:"date"`
	Country string `db:"country"`
	PV      int    `db:"pv"`
	UV      int    `db:"uv"`
}

type FunnelRow struct {
	Stage string `db:"stage"`
	Count int    `db:"count"`
}

func PVUVByDateAndCountry(ctx context.Context, startAt, endAt time.Time) ([]PVUVRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("get engine for PV/UV: %w", err)
	}
	sql := `
		SELECT
			DATE(created_at) AS date,
			country,
			COUNT(*) AS pv,
			COUNT(DISTINCT COALESCE(NULLIF(user_id,''), session_id)) AS uv
		FROM page_views
		WHERE created_at >= ? AND created_at < ?
		GROUP BY DATE(created_at), country
		ORDER BY date ASC, pv DESC
	`
	var rows []PVUVRow
	if err := eng.SelectP(&rows, sql, timefmt.SQLiteDateTime(startAt), timefmt.SQLiteDateTime(endAt)); err != nil {
		return nil, fmt.Errorf("query PV/UV: %w", err)
	}
	return rows, nil
}

func FunnelCounts(ctx context.Context, startAt, endAt time.Time) ([]FunnelRow, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("get engine for funnel: %w", err)
	}

	// Visitors: unique session_id from page_views (anonymous)
	visitorsSQL := `SELECT COUNT(DISTINCT COALESCE(NULLIF(user_id,''), session_id)) FROM page_views WHERE created_at >= ? AND created_at < ?`
	var visitors int
	if err := eng.GetP(&visitors, visitorsSQL, timefmt.SQLiteDateTime(startAt), timefmt.SQLiteDateTime(endAt)); err != nil {
		return nil, fmt.Errorf("query visitors: %w", err)
	}

	// Registered: users created in period
	regSQL := `SELECT COUNT(*) FROM users WHERE created_at >= ? AND created_at < ?`
	var registered int
	if err := eng.GetP(&registered, regSQL, timefmt.SQLiteDateTime(startAt), timefmt.SQLiteDateTime(endAt)); err != nil {
		return nil, fmt.Errorf("query registered: %w", err)
	}

	// Ordered: orders created in period
	orderSQL := `SELECT COUNT(*) FROM orders WHERE created_at >= ? AND created_at < ?`
	var ordered int
	if err := eng.GetP(&ordered, orderSQL, timefmt.SQLiteDateTime(startAt), timefmt.SQLiteDateTime(endAt)); err != nil {
		return nil, fmt.Errorf("query ordered: %w", err)
	}

	// Paid: paid orders in period
	paidSQL := `SELECT COUNT(*) FROM orders WHERE status = 'paid' AND created_at >= ? AND created_at < ?`
	var paid int
	if err := eng.GetP(&paid, paidSQL, timefmt.SQLiteDateTime(startAt), timefmt.SQLiteDateTime(endAt)); err != nil {
		return nil, fmt.Errorf("query paid: %w", err)
	}

	return []FunnelRow{
		{Stage: "visitors", Count: visitors},
		{Stage: "registered", Count: registered},
		{Stage: "ordered", Count: ordered},
		{Stage: "paid", Count: paid},
	}, nil
}
