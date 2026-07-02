package routes

import (
	"time"

	"github.com/labstack/echo/v4"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	fwrequest "github.com/tfnick/go-svelte-starter/api/framework/http/request"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

type CreateUserRequest struct {
	Name  string `json:"name" form:"name"`
	Email string `json:"email" form:"email"`
}

type UpdateUserRequest struct {
	Name  string `json:"name" form:"name"`
	Email string `json:"email" form:"email"`
}

type SetUserActiveRequest struct {
	Active bool `json:"active"`
}

type UserResponse struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	EmailVerified         bool   `json:"email_verified"`
	IsActive              bool   `json:"is_active"`
	IsAdmin               bool   `json:"is_admin"`
	Role                  string `json:"role"`
	OrganizationID        string `json:"organization_id"`
	DataScope             string `json:"data_scope"`
	MembershipLevel       string `json:"membership_level"`
	MembershipExpiresAt   string `json:"membership_expires_at"`
	RegistrationIP        string `json:"registration_ip"`
	RegistrationCountry   string `json:"registration_country"`
	RegistrationRegion    string `json:"registration_region"`
	RegistrationUserAgent string `json:"registration_user_agent"`
	UtmSource             string `json:"utm_source"`
	UtmMedium             string `json:"utm_medium"`
	UtmCampaign           string `json:"utm_campaign"`
	CreatedAt             string `json:"created_at,omitempty"`
	UpdatedAt             string `json:"updated_at,omitempty"`
}

type UsersResponse struct {
	Items      []UserResponse     `json:"items"`
	Pagination PaginationResponse `json:"pagination"`
}

func ToUserResponse(user usecase.UserCo) UserResponse {
	return UserResponse{
		ID:                    user.ID,
		Name:                  user.Name,
		Email:                 user.Email,
		EmailVerified:         user.EmailVerified,
		IsActive:              user.IsActive,
		IsAdmin:               user.IsAdmin,
		Role:                  user.Role,
		OrganizationID:        user.OrganizationID,
		DataScope:             user.DataScope,
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

func ToUserResponses(users []usecase.UserCo) []UserResponse {
	responses := make([]UserResponse, 0, len(users))
	for i := range users {
		responses = append(responses, ToUserResponse(users[i]))
	}
	return responses
}

func ToUsersResponse(users usecase.UsersCo) UsersResponse {
	return UsersResponse{
		Items:      ToUserResponses(users.Items),
		Pagination: ToPaginationResponse(users.Pagination),
	}
}

// GetUserMock mocks a slow user fetch for demo flows.
func GetUserMock(c echo.Context) error {
	time.Sleep(3 * time.Second)
	u := usecase.UserCo{
		ID:       c.Param("id"),
		Name:     "Zach",
		Email:    "email@email.com",
		IsActive: true,
	}
	return httpresponse.OK(c, ToUserResponse(u))
}

// CreateUser creates a user from client-supported request fields.
func CreateUser(c echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	createdUser, err := usecase.CreateUser(ctx, usecase.CreateUserCmd{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.Created(c, ToUserResponse(createdUser))
}

// GetAllUsers returns paginated users.
func GetAllUsers(c echo.Context) error {
	page := fwrequest.PageQuery(c)
	ctx := fwcontext.InternalUsecaseContext(c)
	users, err := usecase.ListUsers(ctx, usecase.ListUsersQry{
		StartTime: c.QueryParam("start_time"),
		EndTime:   c.QueryParam("end_time"),
		Page:      page.Page,
		PageSize:  page.PageSize,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToUsersResponse(users))
}

// GetUser returns one user by path or query id.
func GetUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		id = c.QueryParam("id")
	}
	ctx := fwcontext.InternalUsecaseContext(c)
	user, err := usecase.GetUser(ctx, usecase.UserDetailQry{ID: id})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToUserResponse(user))
}

// UpdateUser updates a user from client-supported request fields.
func UpdateUser(c echo.Context) error {
	id := c.Param("id")
	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	updatedUser, err := usecase.UpdateUser(ctx, usecase.UpdateUserCmd{
		ID:    id,
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserResponse(updatedUser))
}

// SetUserActive enables or disables a user account.
func SetUserActive(c echo.Context) error {
	id := c.Param("id")
	var req SetUserActiveRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	updatedUser, err := usecase.SetUserActive(ctx, usecase.SetUserActiveCmd{
		ID:     id,
		Active: req.Active,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, ToUserResponse(updatedUser))
}

// DeleteUser deletes a user by id.
func DeleteUser(c echo.Context) error {
	id := c.Param("id")
	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.DeleteUser(ctx, usecase.DeleteUserCmd{ID: id}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OKEmpty(c)
}
