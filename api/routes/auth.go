package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	fwauth "github.com/tfnick/go-svelte-starter/api/framework/http/auth"
	fwcontext "github.com/tfnick/go-svelte-starter/api/framework/http/context"
	"github.com/tfnick/go-svelte-starter/api/framework/http/middleware"
	httpresponse "github.com/tfnick/go-svelte-starter/api/framework/http/response"
	"github.com/tfnick/go-svelte-starter/api/framework/logging"
	"github.com/tfnick/go-svelte-starter/api/usecase"
)

var authLogger = logging.For("auth")

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	UtmSource   string `json:"utm_source"`
	UtmMedium   string `json:"utm_medium"`
	UtmCampaign string `json:"utm_campaign"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type OAuthExchangeRequest struct {
	Token string `json:"token"`
}

type CurrentUserResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	EmailVerified       bool     `json:"email_verified"`
	IsAdmin             bool     `json:"is_admin"`
	Role                string   `json:"role"`
	OrganizationID      string   `json:"organization_id"`
	Permissions         []string `json:"permissions"`
	DataScope           string   `json:"data_scope"`
	MembershipLevel     string   `json:"membership_level"`
	MembershipExpiresAt string   `json:"membership_expires_at"`
}

type AuthStatusUserResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	IsAdmin             bool     `json:"is_admin"`
	Role                string   `json:"role"`
	OrganizationID      string   `json:"organization_id"`
	Permissions         []string `json:"permissions"`
	DataScope           string   `json:"data_scope"`
	MembershipLevel     string   `json:"membership_level"`
	MembershipExpiresAt string   `json:"membership_expires_at"`
}

type CurrentUserEnvelope struct {
	User CurrentUserResponse `json:"user"`
}

type AuthTokenResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	TokenType    string              `json:"token_type"`
	ExpiresIn    int64               `json:"expires_in"`
	ExpiresAt    string              `json:"expires_at"`
	User         CurrentUserResponse `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	ExpiresAt   string `json:"expires_at"`
}

type AuthStatusResponse struct {
	LoggedIn bool                    `json:"logged_in"`
	User     *AuthStatusUserResponse `json:"user,omitempty"`
}

func wantsJSON(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get("Content-Type"), "application/json")
}

func bindRegisterRequest(c echo.Context) (RegisterRequest, error) {
	var req RegisterRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Name = c.FormValue("name")
	req.Email = c.FormValue("email")
	req.Password = c.FormValue("password")
	return req, nil
}

func bindLoginRequest(c echo.Context) (LoginRequest, error) {
	var req LoginRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Email = c.FormValue("email")
	req.Password = c.FormValue("password")
	return req, nil
}

func bindForgotPasswordRequest(c echo.Context) (ForgotPasswordRequest, error) {
	var req ForgotPasswordRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Email = c.FormValue("email")
	return req, nil
}

func bindResetPasswordRequest(c echo.Context) (ResetPasswordRequest, error) {
	var req ResetPasswordRequest
	if wantsJSON(c) {
		if err := c.Bind(&req); err != nil {
			return req, err
		}
		return req, nil
	}

	req.Token = c.FormValue("token")
	req.Password = c.FormValue("password")
	return req, nil
}

func bindOAuthExchangeRequest(c echo.Context) (OAuthExchangeRequest, error) {
	var req OAuthExchangeRequest
	if err := c.Bind(&req); err != nil {
		return req, err
	}
	return req, nil
}

func ToCurrentUserResponse(user usecase.UserCo) CurrentUserResponse {
	return CurrentUserResponse{
		ID:                  user.ID,
		Name:                user.Name,
		Email:               user.Email,
		EmailVerified:       user.EmailVerified,
		IsAdmin:             user.IsAdmin,
		Role:                user.Role,
		OrganizationID:      user.OrganizationID,
		Permissions:         append([]string(nil), user.Permissions...),
		DataScope:           user.DataScope,
		MembershipLevel:     user.MembershipLevel,
		MembershipExpiresAt: user.MembershipExpiresAt,
	}
}

func ToAuthStatusUserResponse(user usecase.UserCo) AuthStatusUserResponse {
	return AuthStatusUserResponse{
		ID:                  user.ID,
		Name:                user.Name,
		IsAdmin:             user.IsAdmin,
		Role:                user.Role,
		OrganizationID:      user.OrganizationID,
		Permissions:         append([]string(nil), user.Permissions...),
		DataScope:           user.DataScope,
		MembershipLevel:     user.MembershipLevel,
		MembershipExpiresAt: user.MembershipExpiresAt,
	}
}

func ToAuthStatusResponse(status usecase.AuthStatusCo) AuthStatusResponse {
	if !status.LoggedIn || status.User == nil {
		return AuthStatusResponse{LoggedIn: false}
	}

	user := ToAuthStatusUserResponse(*status.User)
	return AuthStatusResponse{
		LoggedIn: true,
		User:     &user,
	}
}

func ToAuthTokenResponse(auth usecase.AuthCo, accessToken fwauth.Token, refreshToken fwauth.Token) AuthTokenResponse {
	return AuthTokenResponse{
		AccessToken:  accessToken.AccessToken,
		RefreshToken: refreshToken.AccessToken,
		TokenType:    accessToken.TokenType,
		ExpiresIn:    accessToken.ExpiresIn,
		ExpiresAt:    accessToken.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User:         ToCurrentUserResponse(auth.User),
	}
}

func ToRefreshResponse(token fwauth.Token) RefreshResponse {
	return RefreshResponse{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresIn:   token.ExpiresIn,
		ExpiresAt:   token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func issueTokenPair(userID string) (fwauth.Token, fwauth.Token, error) {
	accessToken, err := fwauth.IssueUserToken(userID)
	if err != nil {
		return fwauth.Token{}, fwauth.Token{}, err
	}
	refreshToken, err := fwauth.IssueRefreshToken(userID)
	if err != nil {
		return fwauth.Token{}, fwauth.Token{}, err
	}
	return accessToken, refreshToken, nil
}

func Register(c echo.Context) error {
	req, err := bindRegisterRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	auth, err := usecase.Register(ctx, usecase.RegisterCmd{
		Name:        req.Name,
		Email:       req.Email,
		Password:    req.Password,
		UtmSource:   req.UtmSource,
		UtmMedium:   req.UtmMedium,
		UtmCampaign: req.UtmCampaign,
		Context:     registrationContext(c),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	accessToken, refreshToken, err := issueTokenPair(auth.User.ID)
	if err != nil {
		return httpresponse.InternalServerError(c, err, "failed to issue login token")
	}
	return httpresponse.OK(c, ToAuthTokenResponse(auth, accessToken, refreshToken))
}

func Login(c echo.Context) error {
	req, err := bindLoginRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	auth, err := usecase.Login(ctx, usecase.LoginCmd{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	accessToken, refreshToken, err := issueTokenPair(auth.User.ID)
	if err != nil {
		return httpresponse.InternalServerError(c, err, "failed to issue login token")
	}
	return httpresponse.OK(c, ToAuthTokenResponse(auth, accessToken, refreshToken))
}

func StartOAuthLogin(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	start, err := usecase.StartOAuthLogin(ctx, usecase.OAuthStartCmd{
		Provider:       c.Param("provider"),
		RedirectPath:   c.QueryParam("redirect_path"),
		RequestBaseURL: requestBaseURL(c),
	})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return c.Redirect(http.StatusFound, start.AuthorizationURL)
}

func CompleteOAuthLogin(c echo.Context) error {
	ctx := fwcontext.InternalUsecaseContext(c)
	result, err := usecase.CompleteOAuthLogin(ctx, usecase.OAuthCallbackCmd{
		Provider:       c.Param("provider"),
		Code:           c.QueryParam("code"),
		State:          c.QueryParam("state"),
		RequestBaseURL: requestBaseURL(c),
		Context:        registrationContext(c),
	})
	if err != nil {
		return c.Redirect(http.StatusFound, oauthErrorRedirect(err))
	}

	values := url.Values{}
	values.Set("token", result.ResultToken)
	values.Set("redirect_path", result.RedirectPath)
	return c.Redirect(http.StatusFound, "/app/login/oauth/callback?"+values.Encode())
}

func registrationContext(c echo.Context) usecase.RegistrationContext {
	req := c.Request()
	return usecase.RegistrationContext{
		IP:        c.RealIP(),
		UserAgent: req.UserAgent(),
	}
}

func registrationContextWithUTM(c echo.Context, req RegisterRequest) usecase.RegistrationContext {
	rc := registrationContext(c)
	rc.UtmSource = req.UtmSource
	rc.UtmMedium = req.UtmMedium
	rc.UtmCampaign = req.UtmCampaign
	return rc
}

func ExchangeOAuthLoginResult(c echo.Context) error {
	req, err := bindOAuthExchangeRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	auth, err := usecase.ExchangeOAuthLoginResult(ctx, usecase.OAuthExchangeCmd{Token: req.Token})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	accessToken, refreshToken, err := issueTokenPair(auth.User.ID)
	if err != nil {
		return httpresponse.InternalServerError(c, err, "failed to issue login token")
	}
	return httpresponse.OK(c, ToAuthTokenResponse(auth, accessToken, refreshToken))
}

func RefreshToken(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	token, err := fwauth.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		return httpresponse.InternalUsecaseError(c, fwauth.MapTokenError(err))
	}
	return httpresponse.OK(c, ToRefreshResponse(token))
}

func Logout(c echo.Context) error {
	_ = usecase.Logout(fwcontext.InternalUsecaseContext(c), usecase.LogoutCmd{})
	return httpresponse.OKMessage(c, "logged out")
}

func requestBaseURL(c echo.Context) string {
	req := c.Request()
	scheme := req.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if req.TLS != nil {
			scheme = "https"
		}
	}
	host := req.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = req.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func oauthErrorRedirect(err error) string {
	values := url.Values{}
	values.Set("oauth_error", usecaseMessage(err, "OAuth login failed"))
	return "/app/login?" + values.Encode()
}

func usecaseMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return strings.TrimSpace(fmt.Sprint(err))
}

func GetCurrentUser(c echo.Context) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return httpresponse.Unauthorized(c, "not logged in")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	currentUser, err := usecase.GetCurrentUser(ctx, usecase.CurrentUserQry{UserID: user.ID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OK(c, CurrentUserEnvelope{
		User: ToCurrentUserResponse(currentUser),
	})
}

func ForgotPassword(c echo.Context) error {
	req, err := bindForgotPasswordRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	reset, err := usecase.ForgotPassword(ctx, usecase.ForgotPasswordCmd{Email: req.Email})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	if reset.ResetToken != "" && logging.IsDevelopment() {
		resetURL := fmt.Sprintf("http://%s/reset-password?token=%s", c.Request().Host, reset.ResetToken)
		authLogger.Info().
			Str("user_id", reset.User.ID).
			Str("email", reset.User.Email).
			Str("reset_url", resetURL).
			Msg("password reset link generated for development use")
	}

	return httpresponse.OKMessage(c, "if the email is registered, a reset link has been sent")
}

func ResetPassword(c echo.Context) error {
	req, err := bindResetPasswordRequest(c)
	if err != nil {
		return httpresponse.BadRequest(c, "invalid request data")
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	if err := usecase.ResetPassword(ctx, usecase.ResetPasswordCmd{
		Token:    req.Token,
		Password: req.Password,
	}); err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}

	return httpresponse.OKMessage(c, "password reset successfully, please log in again")
}

func GetAuthStatus(c echo.Context) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return httpresponse.OK(c, AuthStatusResponse{
			LoggedIn: false,
		})
	}

	ctx := fwcontext.InternalUsecaseContext(c)
	status, err := usecase.AuthStatus(ctx, usecase.AuthStatusQry{UserID: user.ID})
	if err != nil {
		return httpresponse.InternalUsecaseError(c, err)
	}
	return httpresponse.OK(c, ToAuthStatusResponse(status))
}
