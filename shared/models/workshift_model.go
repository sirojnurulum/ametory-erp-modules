package models

import (
	"github.com/AMETORY/ametory-erp-modules/shared"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkShiftModel struct {
	shared.BaseModel
	Name        string       `gorm:"type:varchar(255);not null" json:"name"`
	Description string       `gorm:"type:text" json:"description"`
	StartTime   string       `gorm:"type:varchar(255);not null" json:"start_time"`
	EndTime     string       `gorm:"type:varchar(255);not null" json:"end_time"`
	Day         string       `gorm:"type:varchar(255);not null" json:"day"`
	CompanyID   string       `gorm:"type:char(36);index" json:"company_id"` // company id
	Company     CompanyModel `gorm:"foreignKey:CompanyID;constraint:OnDelete:CASCADE"`
}

func (WorkShiftModel) TableName() string {
	return "workshifts"
}
func (w *WorkShiftModel) BeforeCreate(tx *gorm.DB) error {

	if w.ID == "" {
		tx.Statement.SetColumn("id", uuid.New().String())
	}
	return nil
}
