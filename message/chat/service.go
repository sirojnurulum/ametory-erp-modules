package chat

import (
	"errors"
	"net/http"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
)

type ChatService struct {
	db  *gorm.DB
	ctx *context.ERPContext
}

// NewChatService creates a new instance of ChatService.
func NewChatService(db *gorm.DB, ctx *context.ERPContext) *ChatService {
	return &ChatService{ctx: ctx, db: db}
}

// GetChannelByParticipantUserID retrieves channels where a user is a participant.
func (cs *ChatService) GetChannelByParticipantUserID(userID string, request *http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := cs.db.Model(&models.ChatChannelModel{}).
		Joins("JOIN chat_channel_participant_users ON chat_channel_participant_users.chat_channel_model_id = chat_channels.id").
		Where("chat_channel_participant_users.user_model_id = ?", userID)
	if search != "" {
		stmt = stmt.Where("chat_channels.name LIKE ?", "%"+search+"%")
	}
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.ChatChannelModel{})
	page.Page = page.Page + 1
	return page, nil
}

// GetChannelByParticipantMemberID retrieves channels where a member is a participant.
func (cs *ChatService) GetChannelByParticipantMemberID(memberID string, request *http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := cs.db.Model(&models.ChatChannelModel{}).
		Joins("JOIN chat_channel_participant_members ON chat_channel_participant_members.chat_channel_model_id = chat_channels.id").
		Where("chat_channel_participant_members.member_model_id = ?", memberID)
	if search != "" {
		stmt = stmt.Where("chat_channels.name LIKE ?", "%"+search+"%")
	}
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.ChatChannelModel{})
	page.Page = page.Page + 1
	return page, nil
}

// GetChannelDetail retrieves detailed information of a specific channel.
func (cs *ChatService) GetChannelDetail(channelID string) (*models.ChatChannelModel, error) {
	var channel models.ChatChannelModel
	if err := cs.db.
		Preload("ParticipantUsers").
		Preload("ParticipantMembers.User").
		Where("id = ?", channelID).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetChatMessageByChannelID retrieves chat messages by channel ID with pagination.
func (cs *ChatService) GetChatMessageByChannelID(channelID string, request *http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := cs.db.
		Preload("SenderUser").
		Preload("SenderMember.User").
		Where("chat_channel_id = ?", channelID)
	if search != "" {
		stmt = stmt.Where("message ILIKE ?", "%"+search+"%")
	}

	stmt = stmt.Order("created_at DESC")
	if request.URL.Query().Get("order_by") != "" {
		stmt = stmt.Order(request.URL.Query().Get("order_by"))
	}

	stmt = stmt.Model(&models.ChatMessageModel{})
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.ChatMessageModel{})
	page.Page = page.Page + 1
	items := page.Items.(*[]models.ChatMessageModel)
	newItems := make([]models.ChatMessageModel, 0)
	for _, item := range *items {
		item.GetFiles(cs.db)
		newItems = append(newItems, item)
	}
	page.Items = &newItems

	return page, nil
}

// CreateMessage creates a new chat message.
func (cs *ChatService) CreateMessage(messageModel *models.ChatMessageModel) error {
	if messageModel.ChatChannelID == nil {
		return errors.New("chat channel id is required")
	}
	if messageModel.SenderUserID == nil && messageModel.SenderMemberID == nil {
		return errors.New("sender user id or member id is required")
	}

	return cs.db.Create(messageModel).Error
}

// UpdateMessage updates an existing chat message.
func (cs *ChatService) UpdateMessage(messageID string, messageModel *models.ChatMessageModel, userID, memberID *string) error {
	if messageModel.ID == "" {
		return errors.New("message id is required")
	}
	if messageModel.ChatChannelID == nil {
		return errors.New("chat channel id is required")
	}
	if messageModel.SenderUserID == nil && messageModel.SenderMemberID == nil {
		return errors.New("sender user id or member id is required")
	}

	var message models.ChatMessageModel
	if err := cs.db.Model(&message).Where("id = ?", messageID).First(&message).Error; err != nil {
		return err
	}

	if messageModel.RepliesCount > 0 {
		return errors.New("you cannot update message that has replies")
	}

	if userID != nil {
		if message.SenderUserID != userID {
			return errors.New("you are not the sender of this message")
		}
	}
	if memberID != nil {
		if message.SenderMemberID != memberID {
			return errors.New("you are not the sender of this message")
		}
	}

	return cs.db.Save(messageModel).Error
}

// DeleteMessage deletes a chat message.
func (cs *ChatService) DeleteMessage(messageID string, userID, memberID *string) error {
	var message models.ChatMessageModel
	if err := cs.db.Preload("ChatChannel").Where("id = ?", messageID).First(&message).Error; err != nil {
		return err
	}

	if userID != nil && (message.SenderUserID != userID || message.ChatChannel.CreatedByUserID != userID) {
		return errors.New("you are not the sender of this message")
	}
	if memberID != nil && (message.SenderMemberID != memberID || message.ChatChannel.CreatedByMemberID != memberID) {
		return errors.New("you are not the sender of this message")
	}

	return cs.db.Delete(&message).Error
}

// DeleteParticipant removes a participant from a channel.
func (cs *ChatService) DeleteParticipant(channelID string, userID, memberID *string) error {
	var channel models.ChatChannelModel
	if err := cs.db.Model(&channel).Where("id = ?", channelID).First(&channel).Error; err != nil {
		return err
	}

	if userID != nil {
		var user models.UserModel
		cs.db.Model(&user).Where("id = ?", userID).Find(&user)
		cs.db.Model(&channel).Association("ParticipantUsers").Delete(user)
	}

	if memberID != nil {
		var member models.MemberModel
		cs.db.Model(&member).Where("id = ?", memberID).Find(&member)
		cs.db.Model(&channel).Association("ParticipantMembers").Delete(member)
	}

	return nil
}

// AddParticipant adds a participant to a channel.
func (cs *ChatService) AddParticipant(channelID string, userID, memberID *string) error {
	var channel models.ChatChannelModel
	if err := cs.db.Model(&channel).Where("id = ?", channelID).First(&channel).Error; err != nil {
		return err
	}

	if userID != nil {
		var user models.UserModel
		if err := cs.db.Model(&user).Where("id = ?", *userID).First(&user).Error; err != nil {
			return err
		}
		channel.ParticipantUsers = append(channel.ParticipantUsers, &user)
	}

	if memberID != nil {
		var member models.MemberModel
		if err := cs.db.Model(&member).Where("id = ?", *memberID).First(&member).Error; err != nil {
			return err
		}
		channel.ParticipantMembers = append(channel.ParticipantMembers, &member)
	}

	return cs.db.Model(&channel).Updates(channel).Error
}

// CreateChannel creates a new chat channel.
func (cs *ChatService) CreateChannel(channelModel *models.ChatChannelModel, userID, memberID *string) error {
	if channelModel.Name == "" {
		return errors.New("channel name is required")
	}

	if channelModel.CreatedByUserID == nil && channelModel.CreatedByMemberID == nil {
		return errors.New("created by user id or member id is required")
	}

	if userID != nil {
		var user models.UserModel
		if err := cs.db.Model(&user).Where("id = ?", *userID).First(&user).Error; err != nil {
			return err
		}
		channelModel.ParticipantUsers = append(channelModel.ParticipantUsers, &user)
	}
	if memberID != nil {
		var member models.MemberModel
		if err := cs.db.Model(&member).Where("id = ?", *memberID).First(&member).Error; err != nil {
			return err
		}
		channelModel.ParticipantMembers = append(channelModel.ParticipantMembers, &member)
	}

	return cs.db.Create(channelModel).Error
}

// GetDetailMessage retrieves detailed information of a specific chat message.
//
// This function will also load the files and replies of the message.
func (cs *ChatService) GetDetailMessage(messageID string) (*models.ChatMessageModel, error) {
	messageModel := &models.ChatMessageModel{}
	err := cs.db.Where("id = ?", messageID).First(messageModel).Error
	if err != nil {
		return nil, err
	}

	messageModel.GetFiles(cs.db)
	messageModel.GetReplies(cs.db)

	return messageModel, nil
}

// ReadedByMember marks a message as read by a member.
func (cs *ChatService) ReadedByMember(channelID string, memberID string) error {
	err := cs.db.Table("chat_message_read_by_members").
		Where("chat_message_model_id = ? AND member_model_id = ?", channelID, memberID).
		FirstOrCreate(&models.ChatMessageReadByMember{}, models.ChatMessageReadByMember{
			ChatMessageModelID: channelID,
			MemberModelID:      memberID,
		}).Error
	if err != nil {
		return err
	}

	return nil
}

// ReadedByUser marks a message as read by a user.
func (cs *ChatService) ReadedByUser(channelID string, userID string) error {
	err := cs.db.Table("chat_message_read_by_users").
		Where("chat_message_model_id = ? AND user_model_id = ?", channelID, userID).
		FirstOrCreate(&models.ChatMessageReadByUser{}, models.ChatMessageReadByUser{
			ChatMessageModelID: channelID,
			UserModelID:        userID,
		}).Error
	if err != nil {
		return err
	}

	return nil
}

// DeleteChannel deletes a chat channel.
//
// This function will check if the user is the creator of the channel.
func (cs *ChatService) DeleteChannel(channelID string, userID *string, memberID *string) error {
	var channel models.ChatChannelModel
	if err := cs.db.Where("id = ?", channelID).First(&channel).Error; err != nil {
		return err
	}

	if userID != nil {
		if userID != channel.CreatedByUserID {
			return errors.New("you are not the creator of this channel")
		}
	}
	if memberID != nil {
		if memberID != channel.CreatedByUserID {
			return errors.New("you are not the creator of this channel")
		}
	}
	return cs.db.Delete(&channel).Error
}
