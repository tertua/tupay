package queries

import (
	"github.com/google/uuid"
)

// CountClients returns the total clients matching the list filters via the
// shared filter chain, so the count never diverges from the listing.
func (q *ClientQueries) CountClients(orgID uuid.UUID, status, search string) (int64, error) {
	var total int64
	sub := q.filteredClients(orgID, status, search).Select("c.id")
	if err := q.Table("(?) AS client_ids", sub).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
