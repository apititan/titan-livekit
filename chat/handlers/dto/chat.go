package dto

import (
	"github.com/guregu/null"
	"time"
)

type BaseChatDto struct {
	Id                  int64       `json:"id"`
	Name                string      `json:"name"`
	Avatar              null.String `json:"avatar"`
	AvatarBig           null.String `json:"avatarBig"`
	LastUpdateDateTime  time.Time   `json:"lastUpdateDateTime"`
	ParticipantIds      []int64     `json:"participantIds"`
	CanEdit             null.Bool   `json:"canEdit"`
	CanDelete           null.Bool   `json:"canDelete"`
	CanLeave            null.Bool   `json:"canLeave"`
	HasUnreadMessages   bool        `json:"hasUnreadMessages"`
	CanBroadcast        bool        `json:"canBroadcast"`
	CanVideoKick        bool        `json:"canVideoKick"`
	CanChangeChatAdmins bool        `json:"canChangeChatAdmins"`
	IsTetATet           bool        `json:"tetATet"`
	CanAudioMute        bool        `json:"canAudioMute"`
}

type ChatDto struct {
	BaseChatDto
	Participants []*User `json:"participants"`
}

type ChatDtoWithTetATet interface {
	GetId() int64
	GetName() string
	GetIsTetATet() bool
	SetName(s string)
}

func (r *ChatDto) GetId() int64 {
	return r.Id
}

func (r *ChatDto) GetName() string {
	return r.Name
}

func (r *ChatDto) SetName(s string) {
	r.Name = s
}

func (r *ChatDto) GetIsTetATet() bool {
	return r.IsTetATet
}

func (r *BaseChatDto) GetId() int64 {
	return r.Id
}

func (r *BaseChatDto) GetName() string {
	return r.Name
}

func (r *BaseChatDto) SetName(s string) {
	r.Name = s
}

func (r *BaseChatDto) GetIsTetATet() bool {
	return r.IsTetATet
}

// copied view for GET /chat/:id
type ChatDtoWithAdmin struct {
	BaseChatDto
	Participants []*UserWithAdmin `json:"participants"`
}
