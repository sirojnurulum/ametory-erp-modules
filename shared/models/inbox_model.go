package models

import (
	"time"

	"github.com/AMETORY/ametory-erp-modules/shared"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InboxModel adalah model database untuk menampung data inbox
type InboxModel struct {
	shared.BaseModel
	UserID    *string      `gorm:"type:char(36);index" json:"user_id"`
	User      *UserModel   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"user,omitempty"`
	MemberID  *string      `gorm:"type:char(36);index" json:"member_id"`
	Member    *MemberModel `gorm:"foreignKey:MemberID;constraint:OnDelete:CASCADE;" json:"member,omitempty"`
	Name      string       `gorm:"type:varchar(255);default:'INBOX'" json:"name"`
	IsTrash   bool         `gorm:"default:false" json:"is_trash"`
	IsDefault bool         `gorm:"default:false" json:"is_default"`
}

func (InboxModel) TableName() string {
	return "inbox"
}

func (m *InboxModel) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		tx.Statement.SetColumn("id", uuid.New().String())
	}
	return nil
}

// InboxMessageModel adalah model database untuk menampung data inbox message
type InboxMessageModel struct {
	shared.BaseModel
	InboxID              *string             `gorm:"type:char(36);index" json:"inbox_id"`
	Inbox                *InboxModel         `gorm:"foreignKey:InboxID;constraint:OnDelete:CASCADE;" json:"inbox,omitempty"`
	SenderUserID         *string             `gorm:"type:char(36);index" json:"sender_user_id"`
	SenderUser           *UserModel          `gorm:"foreignKey:SenderUserID;constraint:OnDelete:CASCADE;" json:"sender,omitempty"`
	SenderMemberID       *string             `gorm:"type:char(36);index" json:"sender_member_id"`
	SenderMember         *MemberModel        `gorm:"foreignKey:SenderMemberID;constraint:OnDelete:CASCADE;" json:"sender_member,omitempty"`
	RecipientUserID      *string             `gorm:"type:char(36);index" json:"recipient_user_id"`
	RecipientUser        *UserModel          `gorm:"foreignKey:RecipientUserID;constraint:OnDelete:CASCADE;" json:"recipient,omitempty"`
	RecipientMemberID    *string             `gorm:"type:char(36);index" json:"recipient_member_id"`
	RecipientMember      *MemberModel        `gorm:"foreignKey:RecipientMemberID;constraint:OnDelete:CASCADE;" json:"recipient_member,omitempty"`
	CCUsers              []*UserModel        `gorm:"many2many:inbox_message_cc_users;constraint:OnDelete:CASCADE;" json:"cc_users,omitempty"`
	CCMembers            []*MemberModel      `gorm:"many2many:inbox_message_cc_members;constraint:OnDelete:CASCADE;" json:"cc_members,omitempty"`
	Subject              string              `gorm:"type:varchar(255)" json:"subject"`
	Message              string              `gorm:"type:text" json:"message"`
	Read                 bool                `gorm:"type:boolean;default:false" json:"read"`
	ParentInboxMessageID *string             `gorm:"type:char(36);index" json:"parent_inbox_message_id"`
	ParentInboxMessage   *InboxMessageModel  `gorm:"foreignKey:ParentInboxMessageID;constraint:OnDelete:CASCADE;" json:"parent,omitempty"`
	Attachments          []FileModel         `json:"attachments" gorm:"-"`
	Replies              []InboxMessageModel `gorm:"-" json:"replies,omitempty"`
	FavoritedByUsers     []*UserModel        `gorm:"many2many:inbox_message_favorited_by_users;constraint:OnDelete:CASCADE;" json:"favorited_by_users,omitempty"`
	FavoritedByMembers   []*MemberModel      `gorm:"many2many:inbox_message_favorited_by_members;constraint:OnDelete:CASCADE;" json:"favorited_by_members,omitempty"`
	Date                 *time.Time          `json:"date" gorm:"-"`
}

func (InboxMessageModel) TableName() string {
	return "inbox_messages"
}

func (m *InboxMessageModel) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		tx.Statement.SetColumn("id", uuid.New().String())
	}
	return nil
}

func (m *InboxMessageModel) AfterFind(tx *gorm.DB) error {
	if m.Date == nil {
		m.Date = m.CreatedAt
	}
	var files []FileModel
	if err := tx.Where("ref_id = ? AND ref_type = ?", m.ID, "inbox").Find(&files).Error; err == nil {
		m.Attachments = files

	}

	return nil
}

func (m *InboxMessageModel) LoadRecursiveChildren(tx *gorm.DB) ([]InboxMessageModel, error) {
	children := make([]InboxMessageModel, 0)
	if err := tx.Preload("SenderMember.User").
		Preload("SenderUser").
		Preload("RecipientMember.User").
		Preload("RecipientUser").
		Where("parent_inbox_message_id = ?", m.ID).
		Order("created_at").
		Find(&children).Error; err != nil {
		return nil, err
	}

	for _, child := range children {
		subChildren, err := child.LoadRecursiveChildren(tx)
		if err != nil {
			return nil, err
		}
		children = append(children, subChildren...)
	}

	return children, nil
}
