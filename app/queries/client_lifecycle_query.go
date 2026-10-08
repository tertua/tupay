package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
)

// ArchiveClient flips a client's lifecycle status to archived (or back to
// active when archived is false) and reports whether a row matched, so the
// caller can emit a stable 404 for an unknown client.
func (q *ClientQueries) ArchiveClient(orgID, id uuid.UUID, archived bool) (bool, error) {
	status := models.ClientStatusActive
	if archived {
		status = models.ClientStatusArchived
	}
	res := q.Model(&models.Client{}).Where("id = ? AND org_id = ?", id, orgID).
		Updates(map[string]any{
			"updated_at": time.Now(),
			"status":     status,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
