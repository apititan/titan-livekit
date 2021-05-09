package handlers

import (
	"errors"
	"fmt"
	"github.com/getlantern/deepcopy"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/guregu/null"
	"github.com/labstack/echo/v4"
	"net/http"
	"nkonev.name/chat/auth"
	"nkonev.name/chat/client"
	"nkonev.name/chat/db"
	"nkonev.name/chat/handlers/dto"
	. "nkonev.name/chat/logger"
	"nkonev.name/chat/notifications"
	"nkonev.name/chat/utils"
	"strings"
)

type ChatWrapper struct {
	Data  []*dto.ChatDto `json:"data"`
	Count int64          `json:"totalCount"` // total chat number for this user
}

type EditChatDto struct {
	Id int64 `json:"id"`
	CreateChatDto
}

type CreateChatDto struct {
	Name           string  `json:"name"`
	ParticipantIds *[]int64 `json:"participantIds"`
}

type ChatHandler struct {
	db          db.DB
	notificator notifications.Notifications
	restClient  client.RestClient
}

func NewChatHandler(dbR db.DB, notificator notifications.Notifications, restClient client.RestClient) ChatHandler {
	return ChatHandler{db: dbR, notificator: notificator, restClient: restClient}
}

func (a *CreateChatDto) Validate() error {
	return validation.ValidateStruct(a, validation.Field(&a.Name, validation.Required, validation.Length(1, 256)))
}

func (a *EditChatDto) Validate() error {
	return validation.ValidateStruct(a, validation.Field(&a.Name, validation.Required, validation.Length(1, 256)), validation.Field(&a.Id, validation.Required))
}

func (ch ChatHandler) GetChats(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	page := utils.FixPageString(c.QueryParam("page"))
	size := utils.FixSizeString(c.QueryParam("size"))
	offset := utils.GetOffset(page, size)
	searchString := c.QueryParam("searchString")
	searchString = strings.TrimSpace(searchString)

	if dbChats, err := ch.db.GetChatsWithParticipants(userPrincipalDto.UserId, size, offset, searchString, userPrincipalDto); err != nil {
		GetLogEntry(c.Request()).Errorf("Error get chats from db %v", err)
		return err
	} else {
		chatDtos := make([]*dto.ChatDto, 0)
		for _, cc := range dbChats {
			messages, err := ch.db.GetUnreadMessagesCount(cc.Id, userPrincipalDto.UserId)
			if err != nil {
				return err
			}
			cd := convertToDto(cc, []*dto.User{}, messages)
			chatDtos = append(chatDtos, cd)
		}

		var participantIdSet = map[int64]bool{}
		for _, chatDto := range chatDtos {
			for _, participantId := range chatDto.ParticipantIds {
				participantIdSet[participantId] = true
			}
		}
		var users = getUsersRemotelyOrEmpty(participantIdSet, ch.restClient, c)
		for _, chatDto := range chatDtos {
			for _, participantId := range chatDto.ParticipantIds {
				user := users[participantId]
				if user != nil {
					chatDto.Participants = append(chatDto.Participants, user)
				}
			}
		}

		userChatCount, err := ch.db.CountChatsPerUser(userPrincipalDto.UserId)
		if err != nil {
			return errors.New("Error during getting user chat count")
		}
		GetLogEntry(c.Request()).Infof("Successfully returning %v chats", len(chatDtos))
		return c.JSON(http.StatusOK, ChatWrapper{Data: chatDtos, Count: userChatCount})
	}
}

func getChat(dbR db.CommonOperations, restClient client.RestClient, c echo.Context, chatId int64, behalfParticipantId int64, authResult *auth.AuthResult) (*dto.ChatDto, error) {
	if cc, err := dbR.GetChatWithParticipants(behalfParticipantId, chatId); err != nil {
		return nil, err
	} else {
		if cc == nil {
			return nil, nil
		}

		users, err := restClient.GetUsers(cc.ParticipantsIds, c.Request().Context())
		if err != nil {
			users = []*dto.User{}
			GetLogEntry(c.Request()).Warn("Error during getting users from aaa")
		}

		unreadMessages, err := dbR.GetUnreadMessagesCount(cc.Id, behalfParticipantId)
		if err != nil {
			return nil, err
		}
		chatDto := convertToDto(cc, users, unreadMessages)
		if authResult != nil && authResult.HasRole("ROLE_ADMIN") {
			chatDto.CanBroadcast = true
		}
		chatDto.CanVideoKick = cc.IsAdmin
		chatDto.CanChangeChatAdmins = cc.IsAdmin
		return chatDto, nil
	}
}

func (ch ChatHandler) GetChat(c echo.Context) error {
	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	if chat, err := getChat(&ch.db, ch.restClient, c, chatId, userPrincipalDto.UserId, userPrincipalDto); err != nil {
		return err
	} else {
		if chat == nil {
			return c.NoContent(http.StatusNotFound)
		} else {
			var copiedChat = &dto.ChatDtoWithAdmin{}
			err := deepcopy.Copy(copiedChat, chat)
			if err!= nil {
				GetLogEntry(c.Request()).Errorf("error during performing deep copy chat: %s", err)
				return c.NoContent(http.StatusInternalServerError)
			}

			var adminedUsers []*dto.UserWithAdmin
			for _, participant := range copiedChat.Participants {
				var copied  = &dto.UserWithAdmin{}
				if err := deepcopy.Copy(copied, participant); err != nil {
					GetLogEntry(c.Request()).Errorf("error during performing deep copy user: %s", err)
				} else {
					if admin, err := ch.db.IsAdmin(participant.Id, copiedChat.Id); err != nil {
						GetLogEntry(c.Request()).Warnf("Unable to get IsAdmin for user %v in chat %v from db", participant.Id, copiedChat.Id)
					} else {
						copied.Admin = admin
					}
					adminedUsers = append(adminedUsers, copied)
				}
			}
			copiedChat.Participants = adminedUsers

			GetLogEntry(c.Request()).Infof("Successfully returning %v chat", copiedChat)
			return c.JSON(http.StatusOK, copiedChat)
		}
	}
}

func convertToDto(c *db.ChatWithParticipants, users []*dto.User, unreadMessages int64) *dto.ChatDto {
	return &dto.ChatDto{
		Id:                 c.Id,
		Name:               c.Title,
		ParticipantIds:     c.ParticipantsIds,
		Participants:       users,
		CanEdit:            null.BoolFrom(c.IsAdmin),
		LastUpdateDateTime: c.LastUpdateDateTime,
		CanLeave:           null.BoolFrom(!c.IsAdmin),
		UnreadMessages:     unreadMessages,
	}
}

func (ch ChatHandler) CreateChat(c echo.Context) error {
	var bindTo = new(CreateChatDto)
	if err := c.Bind(bindTo); err != nil {
		GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
		return err
	}
	if valid, err := ValidateAndRespondError(c, bindTo); err != nil || !valid {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {
		id, _, err := tx.CreateChat(convertToCreatableChat(bindTo))
		if err != nil {
			return err
		}
		// add admin
		if err := tx.AddParticipant(userPrincipalDto.UserId, id, true); err != nil {
			return err
		}

		if bindTo.ParticipantIds != nil {
			participantIds := *bindTo.ParticipantIds
			// add other participants except admin
			for _, participantId := range participantIds {
				if participantId == userPrincipalDto.UserId {
					continue
				}
				if err := tx.AddParticipant(participantId, id, false); err != nil {
					return err
				}
			}
		}

		responseDto, err := getChat(tx, ch.restClient, c, id, userPrincipalDto.UserId, userPrincipalDto)
		if err != nil {
			return err
		}

		ch.notificator.NotifyAboutNewChat(c, responseDto, responseDto.ParticipantIds, tx)
		return c.JSON(http.StatusCreated, responseDto)
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

func convertToCreatableChat(d *CreateChatDto) *db.Chat {
	return &db.Chat{
		Title: d.Name,
	}
}

func (ch ChatHandler) DeleteChat(c echo.Context) error {
	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {
		if admin, err := tx.IsAdmin(userPrincipalDto.UserId, chatId); err != nil {
			return err
		} else if !admin {
			return errors.New(fmt.Sprintf("User %v is not admin of chat %v", userPrincipalDto.UserId, chatId))
		}
		ids, err := tx.GetParticipantIds(chatId)
		if err != nil {
			return err
		}
		if err := tx.DeleteChat(chatId); err != nil {
			return err
		}
		cd := &dto.ChatDto{
			Id: chatId,
		}
		ch.notificator.NotifyAboutDeleteChat(c, cd, ids, tx)
		return c.JSON(http.StatusAccepted, &utils.H{"id": chatId})
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

func (ch ChatHandler) EditChat(c echo.Context) error {
	var bindTo = new(EditChatDto)
	if err := c.Bind(bindTo); err != nil {
		GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
		return err
	}

	if valid, err := ValidateAndRespondError(c, bindTo); err != nil || !valid {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	var userIdsToNotifyAboutChatCreated []int64
	var userIdsToNotifyAboutChatDeleted []int64
	var userIdsToNotifyAboutChatChanged []int64
	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {
		if admin, err := tx.IsAdmin(userPrincipalDto.UserId, bindTo.Id); err != nil {
			return err
		} else if !admin {
			return errors.New(fmt.Sprintf("User %v is not admin of chat %v", userPrincipalDto.UserId, bindTo.Id))
		}
		_, err := tx.EditChat(bindTo.Id, bindTo.Name)
		if err != nil {
			return err
		}

		existsChatParticipantIdsFromDatabase, err := tx.GetParticipantIds(bindTo.Id)
		if err != nil {
			return err
		}

		if bindTo.ParticipantIds != nil {
			// editing participants
			participantIds := *bindTo.ParticipantIds
			for _, participantIdFromRequest := range participantIds {
				// not exists in database
				if !utils.Contains(existsChatParticipantIdsFromDatabase, participantIdFromRequest) {
					if err := tx.AddParticipant(participantIdFromRequest, bindTo.Id, false); err != nil {
						return err
					}
					userIdsToNotifyAboutChatCreated = append(userIdsToNotifyAboutChatCreated, participantIdFromRequest)
				} else { // exists in database
					userIdsToNotifyAboutChatChanged = append(userIdsToNotifyAboutChatChanged, participantIdFromRequest)
				}
			}
			for _, participantIdFromDatabase := range existsChatParticipantIdsFromDatabase {
				// not present in request array and not myself
				if !utils.Contains(participantIds, participantIdFromDatabase) && participantIdFromDatabase != userPrincipalDto.UserId {
					if err := tx.DeleteParticipant(participantIdFromDatabase, bindTo.Id); err != nil {
						return err
					}
					userIdsToNotifyAboutChatDeleted = append(userIdsToNotifyAboutChatDeleted, participantIdFromDatabase)
				}
			}
		} else {
			// not editing participants - just sending notification about chat (name) change
			if ids, err := tx.GetParticipantIds(bindTo.Id); err != nil {
				return err
			} else {
				for _, pid := range ids {
					userIdsToNotifyAboutChatChanged = append(userIdsToNotifyAboutChatChanged, pid)
				}
			}
		}

		if responseDto, err := getChat(tx, ch.restClient, c, bindTo.Id, userPrincipalDto.UserId, userPrincipalDto); err != nil {
			return err
		} else {
			ch.notificator.NotifyAboutNewChat(c, responseDto, userIdsToNotifyAboutChatCreated, tx)
			ch.notificator.NotifyAboutDeleteChat(c, responseDto, userIdsToNotifyAboutChatDeleted, tx)
			ch.notificator.NotifyAboutChangeChat(c, responseDto, userIdsToNotifyAboutChatChanged, tx)
			return c.JSON(http.StatusAccepted, responseDto)
		}
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

func (ch ChatHandler) LeaveChat(c echo.Context) error {
	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok || userPrincipalDto == nil {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {
		if err := tx.DeleteParticipant(userPrincipalDto.UserId, chatId); err != nil {
			return err
		}

		firstUser, err2 := tx.GetFirstParticipant(chatId)
		if err2 != nil {
			return err2
		}
		if responseDto, err := getChat(tx, ch.restClient, c, chatId, firstUser, nil); err != nil {
			return err
		} else {
			ch.notificator.NotifyAboutChangeChat(c, responseDto, responseDto.ParticipantIds, tx)
			ch.notificator.NotifyAboutDeleteChat(c, responseDto, []int64{userPrincipalDto.UserId}, tx)
			return c.JSON(http.StatusAccepted, responseDto)
		}
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

type ChangeAdminResponseDto struct {
	Admin bool `json:"admin"`
}

func (ch ChatHandler) ChangeParticipant(c echo.Context) error {
	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok || userPrincipalDto == nil {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {

		// check that I am admin
		admin, err := tx.IsAdmin(userPrincipalDto.UserId, chatId)
		if err != nil {
			return err
		}
		if !admin {
			return c.JSON(http.StatusUnauthorized, &utils.H{"message": "You have no access to this chat"})
		}
		interestingUserId, err := GetPathParamAsInt64(c, "participantId")
		participant, err := tx.IsParticipant(interestingUserId, chatId)
		if err != nil {
			return err
		}
		if !participant {
			return c.JSON(http.StatusBadRequest, &utils.H{"message": "User is not belong to chat"})
		}

		newAdmin, err := GetQueryParamAsBoolean(c, "admin")
		if err != nil {
			return err
		}
		err = tx.SetAdmin(interestingUserId, chatId, newAdmin)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during changing chat admin in database %v", err)
			return err
		}
		isAdmin, err := tx.IsAdmin(interestingUserId, chatId)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during getting chat admin in database %v", err)
			return err
		}
		participantIds, err := tx.GetParticipantIds(chatId)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during getting chat participants %v", err)
			return err
		}

		var userIdsToNotifyAboutChatChanged []int64
		for _, participantIdFromRequest := range participantIds {
			userIdsToNotifyAboutChatChanged = append(userIdsToNotifyAboutChatChanged, participantIdFromRequest)
		}
		if responseDto, err := getChat(tx, ch.restClient, c, chatId, userPrincipalDto.UserId, userPrincipalDto); err != nil {
			return err
		} else {
			ch.notificator.NotifyAboutChangeChat(c, responseDto, userIdsToNotifyAboutChatChanged, tx)
		}
		responseDto := ChangeAdminResponseDto{Admin: isAdmin}

		return c.JSON(http.StatusAccepted, responseDto)
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

func (ch ChatHandler) DeleteParticipant(c echo.Context) error {
	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok || userPrincipalDto == nil {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {

		// check that I am admin
		admin, err := tx.IsAdmin(userPrincipalDto.UserId, chatId)
		if err != nil {
			return err
		}
		if !admin {
			return c.JSON(http.StatusUnauthorized, &utils.H{"message": "You have no access to this chat"})
		}
		interestingUserId, err := GetPathParamAsInt64(c, "participantId")

		err = tx.DeleteParticipant(interestingUserId, chatId)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during changing chat admin in database %v", err)
			return err
		}
		participantIds, err := tx.GetParticipantIds(chatId)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during getting chat participants %v", err)
			return err
		}

		var userIdsToNotifyAboutChatChanged []int64
		for _, participantIdFromRequest := range participantIds {
			userIdsToNotifyAboutChatChanged = append(userIdsToNotifyAboutChatChanged, participantIdFromRequest)
		}
		if chatDto, err := getChat(tx, ch.restClient, c, chatId, userPrincipalDto.UserId, userPrincipalDto); err != nil {
			return err
		} else {
			ch.notificator.NotifyAboutChangeChat(c, chatDto, userIdsToNotifyAboutChatChanged, tx)
			ch.notificator.NotifyAboutDeleteChat(c, chatDto, []int64{interestingUserId}, tx)
		}

		return c.NoContent(http.StatusAccepted)
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

type AddParticipantsDto struct {
	ParticipantIds     []int64   `json:"addParticipantIds"`
}

func (ch ChatHandler) AddParticipants(c echo.Context) error {
	chatId, err := GetPathParamAsInt64(c, "id")
	if err != nil {
		return err
	}

	var userPrincipalDto, ok = c.Get(utils.USER_PRINCIPAL_DTO).(*auth.AuthResult)
	if !ok || userPrincipalDto == nil {
		GetLogEntry(c.Request()).Errorf("Error during getting auth context")
		return errors.New("Error during getting auth context")
	}

	errOuter := db.Transact(ch.db, func(tx *db.Tx) error {

		// check that I am admin
		admin, err := tx.IsAdmin(userPrincipalDto.UserId, chatId)
		if err != nil {
			return err
		}
		if !admin {
			return c.JSON(http.StatusUnauthorized, &utils.H{"message": "You have no access to this chat"})
		}
		var bindTo = new(AddParticipantsDto)
		if err := c.Bind(bindTo); err != nil {
			GetLogEntry(c.Request()).Warnf("Error during binding to dto %v", err)
			return err
		}

		for _, participantId := range bindTo.ParticipantIds {
			err = tx.AddParticipant(participantId, chatId, false)
			if err != nil {
				GetLogEntry(c.Request()).Errorf("Error during changing chat admin in database %v", err)
				return err
			}
		}
		participantIds, err := tx.GetParticipantIds(chatId)
		if err != nil {
			GetLogEntry(c.Request()).Errorf("Error during getting chat participants %v", err)
			return err
		}

		var userIdsToNotifyAboutChatChanged []int64
		for _, participantIdFromRequest := range participantIds {
			userIdsToNotifyAboutChatChanged = append(userIdsToNotifyAboutChatChanged, participantIdFromRequest)
		}
		if chatDto, err := getChat(tx, ch.restClient, c, chatId, userPrincipalDto.UserId, userPrincipalDto); err != nil {
			return err
		} else {
			ch.notificator.NotifyAboutChangeChat(c, chatDto, userIdsToNotifyAboutChatChanged, tx)
		}

		return c.NoContent(http.StatusAccepted)
	})
	if errOuter != nil {
		GetLogEntry(c.Request()).Errorf("Error during act transaction %v", errOuter)
	}
	return errOuter
}

func (ch ChatHandler) CheckAccess(c echo.Context) error {
	chatId, err := GetQueryParamAsInt64(c, "chatId")
	if err != nil {
		return err
	}
	userId, err := GetQueryParamAsInt64(c, "userId")
	if err != nil {
		return err
	}
	participant, err := ch.db.IsParticipant(userId, chatId)
	if err != nil {
		return err
	}
	if participant {
		return c.NoContent(http.StatusOK)
	} else {
		return c.NoContent(http.StatusUnauthorized)
	}
}