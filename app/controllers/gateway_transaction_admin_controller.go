package controllers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tertua/tupay/pkg/utils"
)

// ListAllTransactions returns one page of relay transactions for admins.
// @Description List relay transactions.
// @Summary list relay transactions
// @Tags Admin
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 20, max 100)"
// @Success 200 {object} map[string]interface{}
// @Router /admin/gateway/transactions [get]
func ListAllTransactions(c fiber.Ctx) error {
	db, ok := openDB(c)
	if !ok {
		return nil
	}
	paging := utils.ParsePagination(c)
	rows, err := db.ListAllTransactions(paging.Limit(), paging.Offset())
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to load transactions", nil)
	}
	total, err := db.CountAllTransactions()
	if err != nil {
		return utils.Fail(c, fiber.StatusInternalServerError, "failed to count transactions", nil)
	}
	out := make([]intentResponseRow, 0, len(rows))
	for _, t := range rows {
		out = append(out, intentResponse(t))
	}
	return utils.OK(c, fiber.StatusOK, fiber.Map{"transactions": out, "meta": paging.Meta(total)})
}
