package dto

import "github.com/montag451/go-eventbus"

const CHAT_EVENTS = "events.chat"
const GLOBAL_EVENTS = "events.global"

type ChatEvent struct {
	EventType                    string                        `json:"eventType"`
	ChatId                       int64                         `json:"chatId"`
	UserId                       int64                         `json:"userId"`
	MessageNotification          *DisplayMessageDto            `json:"messageNotification"`
	MessageDeletedNotification   *MessageDeletedDto            `json:"messageDeletedNotification"`
	UserTypingNotification       *UserTypingNotification       `json:"userTypingNotification"`
	MessageBroadcastNotification *MessageBroadcastNotification `json:"messageBroadcastNotification"`
}

func (ChatEvent) Name() eventbus.EventName {
	return CHAT_EVENTS
}

type GlobalEvent struct {
	EventType                     string                        `json:"eventType"`
	UserId                        int64                         `json:"userId"`
	ChatNotification              *ChatDtoWithAdmin             `json:"chatNotification"`
	ChatDeletedDto                *ChatDeletedDto           `json:"chatDeletedNotification"`
	UserProfileNotification       *User                         `json:"userProfileNotification"`
	VideoCallUserCountEvent       *VideoCallUserCountChangedDto `json:"videoCallUserCountEvent"`
	VideoChatInvitation           *VideoCallInvitation          `json:"videoCallInvitation"`
	VideoParticipantDialEvent     *VideoDialChanges             `json:"videoParticipantDialEvent"`
	UnreadMessagesNotification    *ChatUnreadMessageChanged     `json:"unreadMessagesNotification"`
	AllUnreadMessagesNotification *AllUnreadMessages            `json:"allUnreadMessagesNotification"`
	VideoCallRecordingEvent       *VideoCallRecordingChangedDto `json:"videoCallRecordingEvent"`
}

func (GlobalEvent) Name() eventbus.EventName {
	return GLOBAL_EVENTS
}
