package dto

type ChatEvent struct {
	EventType                    string                        `json:"eventType"`
	ChatId                       int64                         `json:"chatId"`
	UserId                       int64                         `json:"userId"`
	MessageNotification          *DisplayMessageDto            `json:"messageNotification"`
	MessageDeletedNotification   *MessageDeletedDto            `json:"messageDeletedNotification"`
	UserTypingNotification       *UserTypingNotification       `json:"userTypingNotification"`
	MessageBroadcastNotification *MessageBroadcastNotification `json:"messageBroadcastNotification"`
}

type GlobalEvent struct {
	EventType                     string                    `json:"eventType"`
	UserId                        int64                     `json:"userId"`
	ChatNotification              *ChatDtoWithAdmin         `json:"chatNotification"`
	ChatDeletedDto                *ChatDeletedDto           `json:"chatDeletedNotification"`
	UserProfileNotification       *User                     `json:"userProfileNotification"`
	UnreadMessagesNotification    *ChatUnreadMessageChanged `json:"unreadMessagesNotification"`
	AllUnreadMessagesNotification *AllUnreadMessages        `json:"allUnreadMessagesNotification"`
}
