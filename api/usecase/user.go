package usecase

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tfnick/go-svelte-starter/api/framework/authz"
	"github.com/tfnick/go-svelte-starter/api/framework/data/modelerror"
	"github.com/tfnick/go-svelte-starter/api/framework/timefmt"
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type UserCo struct {
	ID                    string
	Name                  string
	Email                 string
	EmailVerified         bool
	IsActive              bool
	IsAdmin               bool
	Role                  string
	OrganizationID        string
	Permissions           []string
	DataScope             string
	MembershipLevel       string
	MembershipExpiresAt   string
	RegistrationIP        string
	RegistrationCountry   string
	RegistrationRegion    string
	RegistrationUserAgent string
	UtmSource             string
	UtmMedium             string
	UtmCampaign           string
	CreatedAt             string
	UpdatedAt             string
}

type CreateUserCmd struct {
	Name  string
	Email string
}

type UpdateUserCmd struct {
	ID    string
	Name  string
	Email string
}

type DeleteUserCmd struct {
	ID string
}

type SetUserActiveCmd struct {
	ID     string
	Active bool
}

type UserDetailQry struct {
	ID string
}

type ListUsersQry struct {
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

type UsersCo struct {
	Items      []UserCo
	Pagination fwusecase.PageResult
}

func CreateUser(ctx fwusecase.Context, cmd CreateUserCmd) (UserCo, error) {
	if cmd.Name == "" || cmd.Email == "" {
		return UserCo{}, fwusecase.E(fwusecase.CodeValidation, "name and email are required", nil)
	}

	user := &models.User{
		Name:  cmd.Name,
		Email: cmd.Email,
	}
	if err := models.CreateUser(ctx.Std(), user); err != nil {
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to create user", err)
	}

	createdUser, err := models.GetUserByID(ctx.Std(), user.ID)
	if err != nil {
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load user", err)
	}
	return userCoFromModel(createdUser), nil
}

func ListUsers(ctx fwusecase.Context, qry ListUsersQry) (UsersCo, error) {
	pageQuery, err := fwusecase.NormalizePageQuery(fwusecase.PageQuery{
		Page:     qry.Page,
		PageSize: qry.PageSize,
	})
	if err != nil {
		return UsersCo{}, err
	}

	query, err := userListQueryFromQry(qry, pageQuery)
	if err != nil {
		return UsersCo{}, err
	}

	total, err := models.CountUsers(ctx.Std(), query)
	if err != nil {
		return UsersCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to count users", err)
	}

	users, err := models.ListUsers(ctx.Std(), query)
	if err != nil {
		return UsersCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load users", err)
	}

	responses := make([]UserCo, 0, len(users))
	for i := range users {
		responses = append(responses, userCoFromListItem(&users[i]))
	}
	return UsersCo{
		Items:      responses,
		Pagination: fwusecase.NewPageResult(pageQuery, total),
	}, nil
}

func userListQueryFromQry(qry ListUsersQry, pageQuery fwusecase.PageQuery) (models.UserListQuery, error) {
	startTime, err := normalizeUserListTime(qry.StartTime, "start_time")
	if err != nil {
		return models.UserListQuery{}, err
	}
	endTime, err := normalizeUserListTime(qry.EndTime, "end_time")
	if err != nil {
		return models.UserListQuery{}, err
	}
	if startTime != "" && endTime != "" && startTime > endTime {
		return models.UserListQuery{}, fwusecase.E(fwusecase.CodeValidation, "start_time must be before end_time", nil)
	}
	return models.UserListQuery{
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     pageQuery.Limit(),
		Offset:    pageQuery.Offset(),
	}, nil
}

func normalizeUserListTime(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if parsed, err := time.ParseInLocation(timefmt.SQLiteDateTimeLayout, value, time.UTC); err == nil {
		return timefmt.SQLiteDateTime(parsed), nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return timefmt.SQLiteDateTime(parsed), nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return timefmt.SQLiteDateTime(parsed), nil
	}
	return "", fwusecase.E(fwusecase.CodeValidation, fmt.Sprintf("%s is invalid", field), nil)
}

func GetUser(ctx fwusecase.Context, qry UserDetailQry) (UserCo, error) {
	if qry.ID == "" {
		return UserCo{}, fwusecase.E(fwusecase.CodeValidation, "missing user ID", nil)
	}

	user, err := models.GetUserByID(ctx.Std(), qry.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserCo{}, fwusecase.E(fwusecase.CodeNotFound, "user not found", err)
		}
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load user", err)
	}
	return userCoFromModel(user), nil
}

func UpdateUser(ctx fwusecase.Context, cmd UpdateUserCmd) (UserCo, error) {
	if cmd.ID == "" {
		return UserCo{}, fwusecase.E(fwusecase.CodeValidation, "missing user ID", nil)
	}

	user := &models.User{
		ID:    cmd.ID,
		Name:  cmd.Name,
		Email: cmd.Email,
	}
	if err := models.UpdateUser(ctx.Std(), user); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return UserCo{}, fwusecase.E(fwusecase.CodeNotFound, "user not found", err)
		}
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update user", err)
	}

	updatedUser, err := models.GetUserByID(ctx.Std(), cmd.ID)
	if err != nil {
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load user", err)
	}
	return userCoFromModel(updatedUser), nil
}

func SetUserActive(ctx fwusecase.Context, cmd SetUserActiveCmd) (UserCo, error) {
	if cmd.ID == "" {
		return UserCo{}, fwusecase.E(fwusecase.CodeValidation, "missing user ID", nil)
	}
	if !cmd.Active && ctx.Actor.Authenticated && ctx.Actor.UserID == cmd.ID {
		return UserCo{}, fwusecase.E(fwusecase.CodeValidation, "cannot disable current user", nil)
	}

	if err := models.SetUserActive(ctx.Std(), cmd.ID, cmd.Active); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return UserCo{}, fwusecase.E(fwusecase.CodeNotFound, "user not found", err)
		}
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to update user active state", err)
	}

	user, err := models.GetUserByID(ctx.Std(), cmd.ID)
	if err != nil {
		return UserCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load user", err)
	}
	return userCoFromModel(user), nil
}

func DeleteUser(ctx fwusecase.Context, cmd DeleteUserCmd) error {
	if cmd.ID == "" {
		return fwusecase.E(fwusecase.CodeValidation, "missing user ID", nil)
	}
	if err := models.DeleteUser(ctx.Std(), cmd.ID); err != nil {
		if errors.Is(err, modelerror.ErrNotFound) {
			return fwusecase.E(fwusecase.CodeNotFound, "user not found", err)
		}
		return fwusecase.E(fwusecase.CodeNotFound, "user not found", err)
	}
	return nil
}

func userCoFromModel(user *models.User) UserCo {
	if user == nil {
		return UserCo{}
	}
	role := authz.NormalizeRole(user.Role, user.IsAdmin == 1)
	return UserCo{
		ID:                  user.ID,
		Name:                user.Name,
		Email:               user.Email,
		EmailVerified:       user.EmailVerified == 1,
		IsActive:            user.IsActive == 1,
		IsAdmin:             user.IsAdmin == 1,
		Role:                role,
		OrganizationID:      user.OrganizationID,
		Permissions:         authz.PermissionsForRole(role),
		DataScope:           authz.DataScopeForRole(role),
		MembershipLevel:     user.MembershipLevel,
		MembershipExpiresAt: user.MembershipExpiresAt,
		CreatedAt:           user.CreatedAt,
		UpdatedAt:           user.UpdatedAt,
	}
}

func userCoFromListItem(user *models.UserListItem) UserCo {
	if user == nil {
		return UserCo{}
	}
	role := authz.NormalizeRole(user.Role, user.IsAdmin == 1)
	return UserCo{
		ID:                    user.ID,
		Name:                  user.Name,
		Email:                 user.Email,
		EmailVerified:         user.EmailVerified == 1,
		IsActive:              user.IsActive == 1,
		IsAdmin:               user.IsAdmin == 1,
		Role:                  role,
		OrganizationID:        user.OrganizationID,
		Permissions:           authz.PermissionsForRole(role),
		DataScope:             authz.DataScopeForRole(role),
		MembershipLevel:       user.MembershipLevel,
		MembershipExpiresAt:   user.MembershipExpiresAt,
		RegistrationIP:        user.RegistrationIP,
		RegistrationCountry:   user.RegistrationCountry,
		RegistrationRegion:    user.RegistrationRegion,
		RegistrationUserAgent: user.RegistrationUserAgent,
		UtmSource:             user.UtmSource,
		UtmMedium:             user.UtmMedium,
		UtmCampaign:           user.UtmCampaign,
		CreatedAt:             user.CreatedAt,
		UpdatedAt:             user.UpdatedAt,
	}
}
