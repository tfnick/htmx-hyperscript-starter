package usecase

import (
	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
	"github.com/tfnick/go-svelte-starter/api/models"
)

type PointsBalanceQry struct {
	UserID string
}

type AwardOrderPaidPointsCmd struct {
	UserID  string
	OrderID string
	Points  int64
}

type PointsCo struct {
	UserID  string
	Balance int64
}

func GetUserPoints(ctx fwusecase.Context, qry PointsBalanceQry) (PointsCo, error) {
	if qry.UserID == "" {
		return PointsCo{}, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}

	balance, err := models.GetPointBalance(ctx.Std(), qry.UserID)
	if err != nil {
		return PointsCo{}, fwusecase.E(fwusecase.CodeInternal, "failed to load point balance", err)
	}
	return PointsCo{
		UserID:  qry.UserID,
		Balance: balance,
	}, nil
}

func AwardOrderPaidPoints(ctx fwusecase.Context, cmd AwardOrderPaidPointsCmd) (PointsCo, bool, error) {
	if cmd.UserID == "" {
		return PointsCo{}, false, fwusecase.E(fwusecase.CodeValidation, "user ID is required", nil)
	}
	if cmd.OrderID == "" {
		return PointsCo{}, false, fwusecase.E(fwusecase.CodeValidation, "order ID is required", nil)
	}
	if cmd.Points <= 0 {
		return PointsCo{}, false, fwusecase.E(fwusecase.CodeValidation, "points must be greater than 0", nil)
	}

	balance, awarded, err := models.AwardOrderPaidPoints(ctx.Std(), cmd.UserID, cmd.OrderID, cmd.Points)
	if err != nil {
		return PointsCo{}, false, fwusecase.E(fwusecase.CodeInternal, "failed to award points", err)
	}
	return PointsCo{
		UserID:  cmd.UserID,
		Balance: balance,
	}, awarded, nil
}
