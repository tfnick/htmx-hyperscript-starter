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

var ErrDictionaryConflict = errors.New("dictionary conflict")

type DictionaryType struct {
	ID          string `db:"id"`
	TypeKey     string `db:"type_key"`
	Name        string `db:"name"`
	Enabled     int    `db:"enabled"`
	Description string `db:"description"`
	CreatedAt   string `db:"created_at"`
	UpdatedAt   string `db:"updated_at"`
}

type DictionaryValue struct {
	ID               string `db:"id"`
	DictionaryTypeID string `db:"dictionary_type_id"`
	TypeKey          string `db:"type_key"`
	ValueCode        string `db:"value_code"`
	Label            string `db:"label"`
	SortOrder        int    `db:"sort_order"`
	Enabled          int    `db:"enabled"`
	Description      string `db:"description"`
	CreatedAt        string `db:"created_at"`
	UpdatedAt        string `db:"updated_at"`
}

type SaveDictionaryTypeCmd struct {
	ID          string
	TypeKey     string
	Name        string
	Enabled     bool
	Description string
}

type SaveDictionaryValueCmd struct {
	ID               string
	DictionaryTypeID string
	ValueCode        string
	Label            string
	SortOrder        int
	Enabled          bool
	Description      string
}

func ListDictionaryTypes(ctx context.Context) ([]DictionaryType, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	var types []DictionaryType
	if err := d.SelectP(&types, dictionaryTypeSelectSQL()+`
		ORDER BY type_key ASC
	`); err != nil {
		return nil, fmt.Errorf("list dictionary types failed: %w", err)
	}
	return types, nil
}

func CreateDictionaryType(ctx context.Context, cmd SaveDictionaryTypeCmd) (DictionaryType, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryType{}, fmt.Errorf("database unavailable: %w", err)
	}

	dictionaryType := DictionaryType{
		ID:          uuid.Must(uuid.NewV7()).String(),
		TypeKey:     cmd.TypeKey,
		Name:        cmd.Name,
		Enabled:     dictionaryBoolToInt(cmd.Enabled),
		Description: cmd.Description,
	}

	if _, err := d.ExecNamed(`
		INSERT INTO dictionary_types (
			id, type_key, name, enabled, description
		) VALUES (
			:id, :type_key, :name, :enabled, :description
		)
	`, dictionaryType); err != nil {
		if isDictionarySQLiteUniqueConstraint(err) {
			return DictionaryType{}, fmt.Errorf("dictionary type already exists: %w", ErrDictionaryConflict)
		}
		return DictionaryType{}, fmt.Errorf("create dictionary type failed: %w", err)
	}
	return GetDictionaryTypeByID(ctx, dictionaryType.ID)
}

func UpdateDictionaryType(ctx context.Context, cmd SaveDictionaryTypeCmd) (DictionaryType, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryType{}, fmt.Errorf("database unavailable: %w", err)
	}

	dictionaryType := DictionaryType{
		ID:          cmd.ID,
		TypeKey:     cmd.TypeKey,
		Name:        cmd.Name,
		Enabled:     dictionaryBoolToInt(cmd.Enabled),
		Description: cmd.Description,
	}

	result, err := d.ExecNamed(`
		UPDATE dictionary_types SET
			type_key = :type_key,
			name = :name,
			enabled = :enabled,
			description = :description
		WHERE id = :id
	`, dictionaryType)

	if err != nil {
		if isDictionarySQLiteUniqueConstraint(err) {
			return DictionaryType{}, fmt.Errorf("dictionary type already exists: %w", ErrDictionaryConflict)
		}
		return DictionaryType{}, fmt.Errorf("update dictionary type failed: %w", err)
	}
	if err := requireDictionaryRowsAffected(result, "dictionary type not found"); err != nil {
		return DictionaryType{}, err
	}
	return GetDictionaryTypeByID(ctx, cmd.ID)
}

func SetDictionaryTypeEnabled(ctx context.Context, id string, enabled bool) (DictionaryType, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryType{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE dictionary_types
		SET enabled = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, dictionaryBoolToInt(enabled), id)
	if err != nil {
		return DictionaryType{}, fmt.Errorf("set dictionary type enabled failed: %w", err)
	}
	if err := requireDictionaryRowsAffected(result, "dictionary type not found"); err != nil {
		return DictionaryType{}, err
	}
	return GetDictionaryTypeByID(ctx, id)
}

func GetDictionaryTypeByID(ctx context.Context, id string) (DictionaryType, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryType{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := dictionaryTypeSelectSQL() + `
		WHERE id = ?
		LIMIT 1
	`
	var dictionaryType DictionaryType
	if err := d.GetP(&dictionaryType, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DictionaryType{}, fmt.Errorf("dictionary type not found: %w", modelerror.ErrNotFound)
		}
		return DictionaryType{}, fmt.Errorf("get dictionary type failed: %w", err)
	}
	return dictionaryType, nil
}

func ListDictionaryValues(ctx context.Context, dictionaryTypeID string) ([]DictionaryValue, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	query := dictionaryValueSelectSQL() + `
		WHERE dv.dictionary_type_id = ?
		ORDER BY dv.sort_order ASC, dv.value_code ASC
	`
	var values []DictionaryValue
	if err := d.SelectP(&values, query, dictionaryTypeID); err != nil {
		return nil, fmt.Errorf("list dictionary values failed: %w", err)
	}
	return values, nil
}

func ListDictionaryOptions(ctx context.Context, typeKeys []string) (map[string][]DictionaryValue, error) {
	eng, err := db.EngineFor(ctx, "app")
	if err != nil {
		return nil, fmt.Errorf("database unavailable: %w", err)
	}

	result := make(map[string][]DictionaryValue, len(typeKeys))
	for _, typeKey := range typeKeys {
		result[typeKey] = []DictionaryValue{}
	}
	if len(typeKeys) == 0 {
		return result, nil
	}

	type query struct {
		TypeKeys []string `db:"type_keys"`
	}
	sql := dictionaryValueSelectSQL() + `
		WHERE dt.type_key IN :type_keys
			AND dt.enabled = 1
			AND dv.enabled = 1
		ORDER BY dt.type_key ASC, dv.sort_order ASC, dv.value_code ASC
	`

	var values []DictionaryValue
	if err := eng.Select(&values, sql, query{TypeKeys: typeKeys}); err != nil {
		return nil, fmt.Errorf("list dictionary options failed: %w", err)
	}
	for _, value := range values {
		result[value.TypeKey] = append(result[value.TypeKey], value)
	}
	return result, nil
}

func CreateDictionaryValue(ctx context.Context, cmd SaveDictionaryValueCmd) (DictionaryValue, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryValue{}, fmt.Errorf("database unavailable: %w", err)
	}

	value := DictionaryValue{
		ID:               uuid.Must(uuid.NewV7()).String(),
		DictionaryTypeID: cmd.DictionaryTypeID,
		ValueCode:        cmd.ValueCode,
		Label:            cmd.Label,
		SortOrder:        cmd.SortOrder,
		Enabled:          dictionaryBoolToInt(cmd.Enabled),
		Description:      cmd.Description,
	}

	if _, err := d.ExecNamed(`
		INSERT INTO dictionary_values (
			id, dictionary_type_id, value_code, label, sort_order, enabled, description
		) VALUES (
			:id, :dictionary_type_id, :value_code, :label, :sort_order, :enabled, :description
		)
	`, value); err != nil {
		if isDictionarySQLiteConstraint(err) {
			return DictionaryValue{}, fmt.Errorf("dictionary value conflict: %w", ErrDictionaryConflict)
		}
		return DictionaryValue{}, fmt.Errorf("create dictionary value failed: %w", err)
	}
	return GetDictionaryValueByID(ctx, value.ID)
}

func UpdateDictionaryValue(ctx context.Context, cmd SaveDictionaryValueCmd) (DictionaryValue, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryValue{}, fmt.Errorf("database unavailable: %w", err)
	}

	value := DictionaryValue{
		ID:               cmd.ID,
		DictionaryTypeID: cmd.DictionaryTypeID,
		ValueCode:        cmd.ValueCode,
		Label:            cmd.Label,
		SortOrder:        cmd.SortOrder,
		Enabled:          dictionaryBoolToInt(cmd.Enabled),
		Description:      cmd.Description,
	}

	result, err := d.ExecNamed(`
		UPDATE dictionary_values SET
			dictionary_type_id = :dictionary_type_id,
			value_code = :value_code,
			label = :label,
			sort_order = :sort_order,
			enabled = :enabled,
			description = :description
		WHERE id = :id
	`, value)

	if err != nil {
		if isDictionarySQLiteConstraint(err) {
			return DictionaryValue{}, fmt.Errorf("dictionary value conflict: %w", ErrDictionaryConflict)
		}
		return DictionaryValue{}, fmt.Errorf("update dictionary value failed: %w", err)
	}
	if err := requireDictionaryRowsAffected(result, "dictionary value not found"); err != nil {
		return DictionaryValue{}, err
	}
	return GetDictionaryValueByID(ctx, cmd.ID)
}

func SetDictionaryValueEnabled(ctx context.Context, id string, enabled bool) (DictionaryValue, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryValue{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := `
		UPDATE dictionary_values
		SET enabled = ?
		WHERE id = ?
	`
	result, err := d.ExecP(query, dictionaryBoolToInt(enabled), id)
	if err != nil {
		return DictionaryValue{}, fmt.Errorf("set dictionary value enabled failed: %w", err)
	}
	if err := requireDictionaryRowsAffected(result, "dictionary value not found"); err != nil {
		return DictionaryValue{}, err
	}
	return GetDictionaryValueByID(ctx, id)
}

func GetDictionaryValueByID(ctx context.Context, id string) (DictionaryValue, error) {
	d, err := db.EngineFor(ctx, "app")
	if err != nil {
		return DictionaryValue{}, fmt.Errorf("database unavailable: %w", err)
	}

	query := dictionaryValueSelectSQL() + `
		WHERE dv.id = ?
		LIMIT 1
	`
	var value DictionaryValue
	if err := d.GetP(&value, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DictionaryValue{}, fmt.Errorf("dictionary value not found: %w", modelerror.ErrNotFound)
		}
		return DictionaryValue{}, fmt.Errorf("get dictionary value failed: %w", err)
	}
	return value, nil
}

func dictionaryTypeSelectSQL() string {
	return `
		SELECT id, type_key, name, enabled, description, created_at, updated_at
		FROM dictionary_types
	`
}

func dictionaryValueSelectSQL() string {
	return `
		SELECT
			dv.id,
			dv.dictionary_type_id,
			dt.type_key,
			dv.value_code,
			dv.label,
			dv.sort_order,
			dv.enabled,
			dv.description,
			dv.created_at,
			dv.updated_at
		FROM dictionary_values dv
		INNER JOIN dictionary_types dt ON dt.id = dv.dictionary_type_id
	`
}

func dictionaryBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requireDictionaryRowsAffected(result sql.Result, notFoundMessage string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected unavailable: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: %w", notFoundMessage, modelerror.ErrNotFound)
	}
	return nil
}

func isDictionarySQLiteUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}

func isDictionarySQLiteConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "foreign key constraint failed")
}
