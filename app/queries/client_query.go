package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// ClientQueries struct for queries from Client model.
type ClientQueries struct {
	*gorm.DB
}

// GetClient returns one client of an org by ID.
func (q *ClientQueries) GetClient(orgID, id uuid.UUID) (models.Client, error) {
	client := models.Client{}

	if err := q.Where("id = ? AND org_id = ?", id, orgID).First(&client).Error; err != nil {
		return client, notFound(err)
	}

	return client, nil
}

// CreateClient creates a new client; a missing OrgID rides as the creator UserID until callers pass org scope (rides-as).
func (q *ClientQueries) CreateClient(c *models.Client) error {
	if c.OrgID == uuid.Nil {
		c.OrgID = c.UserID
	}
	return q.Create(c).Error
}

// UpdateClient updates a client of an org.
func (q *ClientQueries) UpdateClient(c *models.Client) error {
	if err := q.Model(&models.Client{}).Where("id = ? AND org_id = ?", c.ID, c.OrgID).
		Updates(map[string]any{
			"updated_at": time.Now(),
			"name":       c.Name,
			"email":      c.Email,
			"company":    c.Company,
			"phone":      c.Phone,
			"address":    c.Address,
			"notes":      c.Notes,
		}).Error; err != nil {
		return err
	}

	return nil
}

// DeleteClient deletes a client of an org.
func (q *ClientQueries) DeleteClient(orgID, id uuid.UUID) error {
	if err := q.Where("id = ? AND org_id = ?", id, orgID).Delete(&models.Client{}).Error; err != nil {
		return err
	}

	return nil
}
