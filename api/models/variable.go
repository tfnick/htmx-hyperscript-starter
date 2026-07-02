package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tfnick/go-svelte-starter/api/db"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
)

const (
	VariableValueTypeString  = "string"
	VariableValueTypeNumber  = "number"
	VariableValueTypeBoolean = "boolean"
	VariableValueTypeJSON    = "json"
)

var ErrVariableConflict = errors.New("variable conflict")

type Variable struct {
	ID          string `db:"id"`
	Key         string `db:"variable_key"`
	Name        string `db:"name"`
	ValueType   string `db:"value_type"`
	ValueJSON   string `db:"value_json"`
	Enabled     int    `db:"enabled"`
	Description string `db:"description"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

type SaveVariableCmd struct {
	ID          string
	Key         string
	Name        string
	ValueType   string
	ValueJSON   string
	Enabled     bool
	Description string
}

func ListVariables(ctx context.Context) ([]Variable, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var variables []Variable
	if err := d.SelectP(&variables, variableSelectSQL()+`
		ORDER BY variable_key ASC, created_at DESC
	`); err != nil {
		return nil, fmt.Errorf("list variables failed: %w", err)
	}
	return variables, nil
}

func CreateVariable(ctx context.Context, cmd SaveVariableCmd) (Variable, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return Variable{}, fmt.Errorf("database unavailable: %w", err)
	}

	variable := Variable{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Key:         cmd.Key,
		Name:        cmd.Name,
		ValueType:   cmd.ValueType,
		ValueJSON:   cmd.ValueJSON,
		Enabled:     variableBoolToInt(cmd.Enabled),
		Description: cmd.Description,
	}

	if _, err := d.ExecNamed(`
		INSERT INTO variables (
			id, variable_key, name, value_type, value_json, enabled, description
		) VALUES (
			:id, :variable_key, :name, :value_type, :value_json, :enabled, :description
		)
	`, variable); err != nil {
		if isVariableSQLiteUniqueConstraint(err) {
			return Variable{}, fmt.Errorf("variable already exists: %w", ErrVariableConflict)
		}
		return Variable{}, fmt.Errorf("create variable failed: %w", err)
	}
	return GetVariableByID(ctx, variable.ID)
}

func UpdateVariable(ctx context.Context, cmd SaveVariableCmd) (Variable, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return Variable{}, fmt.Errorf("database unavailable: %w", err)
	}

	variable := Variable{
		ID:          cmd.ID,
		Key:         cmd.Key,
		Name:        cmd.Name,
		ValueType:   cmd.ValueType,
		ValueJSON:   cmd.ValueJSON,
		Enabled:     variableBoolToInt(cmd.Enabled),
		Description: cmd.Description,
	}

	result, err := d.ExecNamed(`
		UPDATE variables SET
			variable_key = :variable_key,
			name = :name,
			value_type = :value_type,
			value_json = :value_json,
			enabled = :enabled,
			description = :description
		WHERE id = :id
	`, variable)

	if err != nil {
		if isVariableSQLiteUniqueConstraint(err) {
			return Variable{}, fmt.Errorf("variable already exists: %w", ErrVariableConflict)
		}
		return Variable{}, fmt.Errorf("update variable failed: %w", err)
	}
	if err := requireVariableRowsAffected(result, "variable not found"); err != nil {
		return Variable{}, err
	}
	return GetVariableByID(ctx, cmd.ID)
}

func SetVariableEnabled(ctx context.Context, id string, enabled bool) (Variable, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return Variable{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE variables
		SET enabled = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, variableBoolToInt(enabled), id)
	if err != nil {
		return Variable{}, fmt.Errorf("set variable enabled failed: %w", err)
	}
	if err := requireVariableRowsAffected(result, "variable not found"); err != nil {
		return Variable{}, err
	}
	return GetVariableByID(ctx, id)
}

func GetVariableByID(ctx context.Context, id string) (Variable, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return Variable{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := variableSelectSQL() + `
		WHERE id = ?
		LIMIT 1
	`
	var variable Variable
	if err := d.GetP(&variable, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Variable{}, fmt.Errorf("variable not found: %w", modelerror.ErrNotFound)
		}
		return Variable{}, fmt.Errorf("get variable failed: %w", err)
	}
	return variable, nil
}

func variableSelectSQL() string {
	return `
		SELECT id, variable_key, name, value_type, value_json, enabled, description, created_at, updated_at
		FROM variables
	`
}

func variableBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireVariableRowsAffected(result sql.Result, notFoundMessage string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected unavailable: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: %w", notFoundMessage, modelerror.ErrNotFound)
	}
	return nil
}

func isVariableSQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed")
}
