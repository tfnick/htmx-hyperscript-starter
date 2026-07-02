package routes

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/db"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
)

const maxSQLRows = 100
const sqlQueryTimeout = 30 * time.Second

// ExperimentSQLQueryRequest is the request body for executing a SQL query.
type ExperimentSQLQueryRequest struct {
	Query string `json:"query"`
}

// ExperimentSQLQueryResponse is the response body containing query results.
type ExperimentSQLQueryResponse struct {
	Columns  []string   `json:"columns"`
	Rows     [][]string `json:"rows"`
	RowCount int        `json:"row_count"`
}

// RunExperimentSQLQuery executes a read-only SELECT query against the app database.
// Only SELECT queries are allowed. Results are limited to maxSQLRows.
// POST /api/admin/experiments/sql-query
func RunExperimentSQLQuery(c echo.Context) error {
	var req ExperimentSQLQueryRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	query := strings.TrimSpace(req.Query)

	if query == "" {
		return httpresponse.BadRequest(c, "query must not be empty")
	}

	// Only allow SELECT queries.
	if !strings.HasPrefix(strings.ToUpper(query), "SELECT") {
		return httpresponse.BadRequest(c, "only SELECT queries are allowed")
	}

	// Reject multi-statement injection: split on semicolons (naive but effective for accidental misuse).
	statements := strings.Split(query, ";")
	var nonEmpty []string
	for _, s := range statements {
		if strings.TrimSpace(s) != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	if len(nonEmpty) > 1 {
		return httpresponse.BadRequest(c, "only a single SELECT query is allowed")
	}

	// Also reject dangerous keywords even if the statement starts with SELECT.
	upperQuery := strings.ToUpper(query)
	for _, keyword := range []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ",
		"CREATE ", "TRUNCATE ", "EXEC ", "EXECUTE ", "MERGE ",
		"CALL ", "LOAD ", "REPLACE ", "GRANT ", "REVOKE ",
	} {
		if strings.Contains(upperQuery, keyword) {
			return httpresponse.BadRequest(c, "only SELECT queries are allowed")
		}
	}

	ucCtx := fwcontext.InternalUsecaseContext(c)
	stdCtx := ucCtx.Std()

	engine, err := db.EngineFor(stdCtx, "app")
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	sqlDB := engine.StdDB()
	if sqlDB == nil {
		return httpresponse.InternalUsecaseError(c, echo.ErrInternalServerError)
	}

	queryCtx, cancel := context.WithTimeout(stdCtx, sqlQueryTimeout)
	defer cancel()

	// Always append LIMIT to enforce row cap, wrapping the user's query.
	limitedQuery := wrapQueryWithLimit(query, maxSQLRows)

	result, err := runSQLQuery(queryCtx, sqlDB, limitedQuery)
	if err != nil {
		if queryCtx.Err() == context.DeadlineExceeded {
			return httpresponse.BadRequest(c, "query timed out after 30 seconds")
		}
		return httpresponse.BadRequest(c, "query execution failed: "+err.Error())
	}

	return httpresponse.OK(c, result)
}

// wrapQueryWithLimit wraps a SELECT query with an outer LIMIT to ensure
// no more than limit rows are returned, even if the user adds their own LIMIT.
func wrapQueryWithLimit(query string, limit int) string {
	upper := strings.ToUpper(query)
	if strings.Contains(upper, "LIMIT") {
		// If the user already has a LIMIT, use a subquery wrapping to enforce our cap.
		return wrapSubqueryWithLimit(query, limit)
	}
	return query + " LIMIT " + itoa(limit)
}

// wrapSubqueryWithLimit wraps a query with an outer LIMIT using a subquery.
func wrapSubqueryWithLimit(query string, limit int) string {
	return "SELECT * FROM (" + query + ") LIMIT " + itoa(limit)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// runSQLQuery executes the query and returns columns and rows as strings.
func runSQLQuery(ctx context.Context, db *sql.DB, query string) (*ExperimentSQLQueryResponse, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result [][]string
	for rows.Next() {
		// Create a slice of interface{} to hold each column value.
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make([]string, len(columns))
		for i, val := range values {
			row[i] = formatSQLValue(val)
		}
		result = append(result, row)

		if len(result) >= maxSQLRows {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &ExperimentSQLQueryResponse{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, nil
}

// formatSQLValue converts a scanned SQL value to its string representation.
func formatSQLValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case string:
		return v
	case int64:
		return itoa64(v)
	case float64:
		return fmtFloat(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func itoa64(n int64) string {
	return fmt.Sprintf("%d", n)
}

func fmtFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
