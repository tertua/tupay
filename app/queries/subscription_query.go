package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// SubscriptionQueries owns subscription persistence: header + items CRUD,
// org-scoped like ItemQueries. It is the only place subscriptions are written.
type SubscriptionQueries struct {
	*gorm.DB
}

// ListSubscriptions returns one page of subscriptions owned by an org, newest first.
func (q *SubscriptionQueries) ListSubscriptions(orgID uuid.UUID, limit, offset int) ([]models.Subscription, error) {
	out := []models.Subscription{}
	if err := q.Where("org_id = ?", orgID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return out, err
	}
	return out, nil
}

// CountSubscriptions returns the total subscriptions of an org.
func (q *SubscriptionQueries) CountSubscriptions(orgID uuid.UUID) (int64, error) {
	var total int64
	if err := q.Model(&models.Subscription{}).Where("org_id = ?", orgID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// GetSubscription returns one subscription owned by an org.
func (q *SubscriptionQueries) GetSubscription(orgID, id uuid.UUID) (models.Subscription, error) {
	sub := models.Subscription{}
	if err := q.Where("id = ? AND org_id = ?", id, orgID).First(&sub).Error; err != nil {
		return sub, notFound(err)
	}
	return sub, nil
}

// GetSubscriptionItems returns the lines of a subscription ordered by position.
func (q *SubscriptionQueries) GetSubscriptionItems(subscriptionID uuid.UUID) ([]models.SubscriptionItem, error) {
	items := []models.SubscriptionItem{}
	if err := q.Where("subscription_id = ?", subscriptionID).Order("position ASC").Find(&items).Error; err != nil {
		return items, err
	}
	return items, nil
}

// CreateSubscription persists a subscription header and its items in one
// transaction (an unscoped OrgID rides as the creator UserID via rideOrg).
func (q *SubscriptionQueries) CreateSubscription(sub *models.Subscription, items []models.SubscriptionItem) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if sub.OrgID == uuid.Nil {
				sub.OrgID = sub.UserID
			}
			if err := tx.Create(sub).Error; err != nil {
				return err
			}
			for i := range items {
				items[i].SubscriptionID = sub.ID
			}
			if len(items) > 0 {
				if err := tx.Create(&items).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// UpdateSubscription replaces a subscription header and its items in one transaction.
func (q *SubscriptionQueries) UpdateSubscription(orgID uuid.UUID, sub *models.Subscription, items []models.SubscriptionItem) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&models.Subscription{}).Where("id = ? AND org_id = ?", sub.ID, orgID).
				Updates(map[string]any{
					"updated_at":     time.Now(),
					"client_id":      sub.ClientID,
					"name":           sub.Name,
					"status":         sub.Status,
					"cadence":        sub.Cadence,
					"next_run_date":  sub.NextRunDate,
					"lead_days":      sub.LeadDays,
					"currency":       sub.Currency,
					"tax_rate":       sub.TaxRate,
					"discount":       sub.Discount,
					"notes":          sub.Notes,
					"terms":          sub.Terms,
					"payment_method": sub.PaymentMethod,
					"invoice_status": sub.InvoiceStatus,
					"due_days":       sub.DueDays,
				}).Error; err != nil {
				return err
			}
			if err := tx.Where("subscription_id = ?", sub.ID).Delete(&models.SubscriptionItem{}).Error; err != nil {
				return err
			}
			for i := range items {
				items[i].SubscriptionID = sub.ID
			}
			if len(items) > 0 {
				if err := tx.Create(&items).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// UpdateSubscriptionStatus updates only the active/paused status of a subscription.
func (q *SubscriptionQueries) UpdateSubscriptionStatus(orgID, id uuid.UUID, status string) error {
	return q.Model(&models.Subscription{}).Where("id = ? AND org_id = ?", id, orgID).
		Updates(map[string]any{
			"updated_at": time.Now(),
			"status":     status,
		}).Error
}

// DeleteSubscription deletes a subscription and its items of an org.
func (q *SubscriptionQueries) DeleteSubscription(orgID, id uuid.UUID) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("subscription_id = ?", id).Delete(&models.SubscriptionItem{}).Error; err != nil {
				return err
			}
			return tx.Where("id = ? AND org_id = ?", id, orgID).Delete(&models.Subscription{}).Error
		})
	})
}

// CountSubscriptionRuns returns how many claim rows reference a subscription;
// the delete guard uses it to refuse a subscription that already generated
// invoices.
func (q *SubscriptionQueries) CountSubscriptionRuns(subscriptionID uuid.UUID) (int64, error) {
	var total int64
	if err := q.Model(&models.SubscriptionRun{}).Where("subscription_id = ?", subscriptionID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
