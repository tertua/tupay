package queries

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
)

// clientSortColumns whitelists sortable columns for client listing. The
// aggregate aliases (total_billed, outstanding) are sorted at the outer query
// only, where both SQLite and PostgreSQL accept a select alias in ORDER BY.
var clientSortColumns = map[string]string{
	"name":         "c.name",
	"created_at":   "c.created_at",
	"total_billed": "total_billed",
	"outstanding":  "outstanding",
}

// ListClients returns one page of clients of an org with billing aggregates,
// filtered by free-text search and lifecycle status and ordered by a
// whitelisted sort column.
func (q *ClientQueries) ListClients(orgID uuid.UUID, search, status, sort, order string, limit, offset int) ([]models.ClientListRow, error) {
	clients := []models.ClientListRow{}

	tx := q.filteredClients(orgID, status, search)

	sortColumn := "c.created_at"
	if column, ok := clientSortColumns[strings.ToLower(sort)]; ok {
		sortColumn = column
	}
	sortOrder := "DESC"
	if strings.EqualFold(order, "asc") {
		sortOrder = "ASC"
	}
	// A stable secondary key keeps pagination deterministic across ties.
	tx = tx.Order(sortColumn + " " + sortOrder).Order("c.created_at DESC")

	if err := tx.Limit(limit).Offset(offset).Scan(&clients).Error; err != nil {
		return clients, err
	}

	return clients, nil
}
