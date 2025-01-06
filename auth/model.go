package auth

import (
	"time"

	"github.com/AMETORY/ametory-erp-modules/company"
	"github.com/AMETORY/ametory-erp-modules/distribution/distributor"
	"github.com/AMETORY/ametory-erp-modules/shared"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserModel adalah model database untuk user
type UserModel struct {
	utils.BaseModel
	FullName                   string                         `gorm:"not null" json:"full_name"`
	Username                   string                         `gorm:"unique" json:"username"`
	Email                      string                         `gorm:"unique;not null" json:"email"`
	Password                   string                         `gorm:"not null" json:"-"`
	VerifiedAt                 *time.Time                     `gorm:"index" json:"verified_at"`
	VerificationToken          string                         `json:"verification_token"`
	VerificationTokenExpiredAt *time.Time                     `gorm:"index" json:"verification_token_expired_at"`
	Roles                      []RoleModel                    `gorm:"many2many:user_roles;" json:"roles"`
	Companies                  []company.CompanyModel         `gorm:"many2many:user_companies;" json:"companies"`
	Distributors               []distributor.DistributorModel `gorm:"many2many:user_distributors;" json:"distributors"`
	ProfilePicture             *shared.FileModel              `json:"profile_picture" gorm:"-"`
}

func (u *UserModel) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		tx.Statement.SetColumn("id", uuid.New().String())
	}
	return
}

func (UserModel) TableName() string {
	return "users"
}

// HashPassword mengenkripsi password menggunakan bcrypt
func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// CheckPassword memverifikasi password dengan hash yang tersimpan
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func (s *AuthService) Migrate() error {

	return s.db.AutoMigrate(&UserModel{}, &RoleModel{}, &PermissionModel{})
}

func (s *AuthService) DB() *gorm.DB {
	return s.db
}
