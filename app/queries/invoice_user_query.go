package queries

import (
	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
)

// CountInvoicesByUser returns how many invoice records name the user as
// creator, across every status and org. Any record — even a draft — blocks an
// admin user delete, since invoice rows stay on the books after their creator
// is gone.
func (q *InvoiceQueries) CountInvoicesByUser(userID uuid.UUID) (int64, error) {
	var count int64
	if err := q.Model(&models.Invoice{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
