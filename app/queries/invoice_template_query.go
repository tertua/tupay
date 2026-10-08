package queries

import (
	"time"

	"github.com/google/uuid"
	"github.com/tertua/tupay/app/models"
	"gorm.io/gorm"
)

// InvoiceTemplateQueries owns invoice-template persistence: header + items CRUD,
// org-scoped like ItemQueries. It is the only place templates are written.
type InvoiceTemplateQueries struct {
	*gorm.DB
}

// ListInvoiceTemplates returns one page of templates owned by an org, newest first.
func (q *InvoiceTemplateQueries) ListInvoiceTemplates(orgID uuid.UUID, limit, offset int) ([]models.InvoiceTemplate, error) {
	out := []models.InvoiceTemplate{}
	if err := q.Where("org_id = ?", orgID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&out).Error; err != nil {
		return out, err
	}
	return out, nil
}

// CountInvoiceTemplates returns the total templates of an org.
func (q *InvoiceTemplateQueries) CountInvoiceTemplates(orgID uuid.UUID) (int64, error) {
	var total int64
	if err := q.Model(&models.InvoiceTemplate{}).Where("org_id = ?", orgID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// GetInvoiceTemplate returns one template owned by an org.
func (q *InvoiceTemplateQueries) GetInvoiceTemplate(orgID, id uuid.UUID) (models.InvoiceTemplate, error) {
	tpl := models.InvoiceTemplate{}
	if err := q.Where("id = ? AND org_id = ?", id, orgID).First(&tpl).Error; err != nil {
		return tpl, notFound(err)
	}
	return tpl, nil
}

// GetInvoiceTemplateItems returns the lines of a template ordered by position.
func (q *InvoiceTemplateQueries) GetInvoiceTemplateItems(templateID uuid.UUID) ([]models.InvoiceTemplateItem, error) {
	items := []models.InvoiceTemplateItem{}
	if err := q.Where("template_id = ?", templateID).Order("position ASC").Find(&items).Error; err != nil {
		return items, err
	}
	return items, nil
}

// CreateInvoiceTemplate persists a template header and its items in one
// transaction (an unscoped OrgID rides as the creator UserID via rideOrg).
func (q *InvoiceTemplateQueries) CreateInvoiceTemplate(tpl *models.InvoiceTemplate, items []models.InvoiceTemplateItem) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if tpl.OrgID == uuid.Nil {
				tpl.OrgID = tpl.UserID
			}
			if err := tx.Create(tpl).Error; err != nil {
				return err
			}
			for i := range items {
				items[i].TemplateID = tpl.ID
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

// UpdateInvoiceTemplate replaces a template header and its items in one transaction.
func (q *InvoiceTemplateQueries) UpdateInvoiceTemplate(orgID uuid.UUID, tpl *models.InvoiceTemplate, items []models.InvoiceTemplateItem) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&models.InvoiceTemplate{}).Where("id = ? AND org_id = ?", tpl.ID, orgID).
				Updates(map[string]any{
					"updated_at":     time.Now(),
					"client_id":      tpl.ClientID,
					"name":           tpl.Name,
					"status":         tpl.Status,
					"cadence":        tpl.Cadence,
					"next_run_date":  tpl.NextRunDate,
					"lead_days":      tpl.LeadDays,
					"currency":       tpl.Currency,
					"tax_rate":       tpl.TaxRate,
					"discount":       tpl.Discount,
					"notes":          tpl.Notes,
					"terms":          tpl.Terms,
					"payment_method": tpl.PaymentMethod,
					"invoice_status": tpl.InvoiceStatus,
					"due_days":       tpl.DueDays,
				}).Error; err != nil {
				return err
			}
			if err := tx.Where("template_id = ?", tpl.ID).Delete(&models.InvoiceTemplateItem{}).Error; err != nil {
				return err
			}
			for i := range items {
				items[i].TemplateID = tpl.ID
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

// UpdateTemplateStatus updates only the active/paused status of a template.
func (q *InvoiceTemplateQueries) UpdateTemplateStatus(orgID, id uuid.UUID, status string) error {
	return q.Model(&models.InvoiceTemplate{}).Where("id = ? AND org_id = ?", id, orgID).
		Updates(map[string]any{
			"updated_at": time.Now(),
			"status":     status,
		}).Error
}

// DeleteInvoiceTemplate deletes a template and its items of an org.
func (q *InvoiceTemplateQueries) DeleteInvoiceTemplate(orgID, id uuid.UUID) error {
	return DoRetry(func() error {
		return q.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("template_id = ?", id).Delete(&models.InvoiceTemplateItem{}).Error; err != nil {
				return err
			}
			return tx.Where("id = ? AND org_id = ?", id, orgID).Delete(&models.InvoiceTemplate{}).Error
		})
	})
}

// CountTemplateRuns returns how many claim rows reference a template; the
// delete guard uses it to refuse a template that already generated invoices.
func (q *InvoiceTemplateQueries) CountTemplateRuns(templateID uuid.UUID) (int64, error) {
	var total int64
	if err := q.Model(&models.InvoiceTemplateRun{}).Where("template_id = ?", templateID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
