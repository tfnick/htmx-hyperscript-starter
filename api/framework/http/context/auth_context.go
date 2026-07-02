package fwcontext

import (
	"github.com/labstack/echo/v4"
	"github.com/tfnick/go-svelte-starter/api/models"
)

const UserContextKey = "user"

func SetCurrentUser(c echo.Context, user *models.User) {
	c.Set(UserContextKey, user)
}

func GetCurrentUser(c echo.Context) *models.User {
	user, ok := c.Get(UserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}
