package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/centrifugal/centrifuge"
	"github.com/centrifugal/protocol"
	"github.com/m7shapan/njson"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"nkonev.name/chat/db"
	"nkonev.name/chat/handlers/dto"
	. "nkonev.name/chat/logger"
	"nkonev.name/chat/redis"
	"nkonev.name/chat/utils"
	"strings"
	"time"
)

func handleLog(e centrifuge.LogEntry) {
	Logger.Printf("%s: %v", e.Message, e.Fields)
}

func getChanPresenceStats(engine centrifuge.Engine, client *centrifuge.Client, e interface{}) *centrifuge.PresenceStats {
	var channel string
	switch v := e.(type) {
	case centrifuge.SubscribeEvent:
		channel = v.Channel
		break
	case centrifuge.UnsubscribeEvent:
		channel = v.Channel
		break
	default:
		Logger.Errorf("Unknown type of event")
		return nil
	}
	stats, err := engine.PresenceStats(channel)
	if err != nil {
		Logger.Errorf("Error during get stats %v", err)
	}
	Logger.Printf("client id=%v, userId=%v acting with channel %s, channelStats.NumUsers %v", client.ID(), client.UserID(), channel, stats.NumUsers)
	return &stats
}

func createPresence(credso *centrifuge.Credentials, client *centrifuge.Client) (*protocol.ClientInfo, time.Duration, error) {
	expiresInString := utils.SecondsToStringMilliseconds(credso.ExpireAt) // to milliseconds for put into dateparse.ParseLocal
	t, err0 := dateparse.ParseLocal(expiresInString)
	if err0 != nil {
		return nil, 0, err0
	}

	presenceDuration := t.Sub(time.Now())
	Logger.Debugf("Calculated session duration %v for credentials %v", presenceDuration, credso)

	clientInfo := &protocol.ClientInfo{
		User:   client.UserID(),
		Client: client.ID(),
	}
	Logger.Infof("Created ClientInfo(Client: %v, UserId: %v)", client.ID(), client.UserID())
	return clientInfo, presenceDuration, nil
}

type PassData struct {
	Payload  utils.H `json:"payload"`
	Metadata utils.H `json:"metadata"`
}

type TypedMessage struct {
	Type string `json:"type"`
}

type MessageRead struct {
	ChatId     int64      `njson:"payload.chatId"`
	MessageId     int64      `njson:"payload.messageId"`
}

// clientId - temporary (session?) UUID, generated by centrifuge
// userId - permanent user id stored in database
func modifyMessage(msg []byte, originatorUserId string, originatorClientId string) ([]byte, error) {
	var v = &PassData{}
	if err := json.Unmarshal(msg, v); err != nil {
		return nil, err
	}
	v.Metadata = utils.H{"originatorUserId": originatorUserId, "originatorClientId": originatorClientId}
	return json.Marshal(v)
}

func ConfigureCentrifuge(lc fx.Lifecycle, dbs db.DB, onlineStorage redis.OnlineStorage) *centrifuge.Node {
	// We use default config here as starting point. Default config contains
	// reasonable values for available options.
	cfg := centrifuge.DefaultConfig
	// In this example we want client to do all possible actions with server
	// without any authentication and authorization. Insecure flag DISABLES
	// many security related checks in library. This is only to make example
	// short. In real app you most probably want authenticate and authorize
	// access to server. See godoc and examples in repo for more details.
	cfg.ClientInsecure = false
	// By default clients can not publish messages into channels. Setting this
	// option to true we allow them to publish.
	cfg.Publish = true

	// Centrifuge library exposes logs with different log level. In your app
	// you can set special function to handle these log entries in a way you want.
	cfg.LogLevel = centrifuge.LogLevelDebug
	cfg.LogHandler = handleLog

	cfg.UserSubscribeToPersonal = true

	// Node is the core object in Centrifuge library responsible for many useful
	// things. Here we initialize new Node instance and pass config to it.
	node, _ := centrifuge.New(cfg)

	redisHost := viper.GetString("centrifuge.redis.host")
	redisPort := viper.GetInt("centrifuge.redis.port")
	redisPassword := viper.GetString("centrifuge.redis.password")
	redisDB := viper.GetInt("centrifuge.redis.db")
	readTimeout := viper.GetDuration("centrifuge.redis.readTimeout")
	writeTimeout := viper.GetDuration("centrifuge.redis.writeTimeout")
	connectTimeout := viper.GetDuration("centrifuge.redis.connectTimeout")
	idleTimeout := viper.GetDuration("centrifuge.redis.idleTimeout")

	redisConf := centrifuge.RedisShardConfig{
		Host:           redisHost,
		Port:           redisPort,
		DB:             redisDB,
		Password:       redisPassword,
		Prefix:         "centrifuge",
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		ConnectTimeout: connectTimeout,
		IdleTimeout:    idleTimeout,
	}
	rec := centrifuge.RedisEngineConfig{
		UseStreams:          false,
		PublishOnHistoryAdd: true,
		HistoryMetaTTL:      300 * time.Second,
		Shards:              []centrifuge.RedisShardConfig{redisConf},
	}
	engine, _ := centrifuge.NewRedisEngine(node, rec)
	node.SetEngine(engine)

	// ClientConnected node event handler is a point where you generally create a
	// binding between Centrifuge and your app business logic. Callback function you
	// pass here will be called every time new connection established with server.
	// Inside this callback function you can set various event handlers for connection.
	node.On().ClientConnected(func(ctx context.Context, client *centrifuge.Client) {
		// Set Subscribe Handler to react on every channel subscription attempt
		// initiated by client. Here you can theoretically return an error or
		// disconnect client from server if needed. But now we just accept
		// all subscriptions.
		var creds, ok = centrifuge.GetCredentials(ctx)
		if !ok {
			Logger.Infof("Cannot extract credentials")
			return
		}
		Logger.Infof("Connected websocket centrifuge client hasCredentials %v, credentials %v", ok, creds)
		userId, err := utils.ParseInt64(creds.UserID)
		if err != nil {
			Logger.Errorf("Unable to parse userId from %v", creds.UserID)
			return
		}
		onlineStorage.PutUserOnline(userId)
		notifyAboutOnline(node, userId, true)

		client.On().Subscribe(func(e centrifuge.SubscribeEvent) centrifuge.SubscribeReply {
			clientInfo, presenceDuration, err := createPresence(creds, client)
			if err != nil {
				Logger.Errorf("Error during creating presence %v", err)
				return centrifuge.SubscribeReply{Error: centrifuge.ErrorInternal}
			}
			channelId, channelName, err := getChannelId(e.Channel)
			if err != nil {
				Logger.Errorf("Error getting channel id %v", err)
				return centrifuge.SubscribeReply{Error: centrifuge.ErrorInternal}
			}
			Logger.Infof("Get channel id %v, channel name %v", channelId, channelName)

			err = checkPermissions(dbs, creds.UserID, channelId, channelName)
			if err != nil {
				Logger.Errorf("Error during checking permissions userId %v, channelId %v, channelName %v,", creds.UserID, channelId, channelName)
				return centrifuge.SubscribeReply{Error: centrifuge.ErrorPermissionDenied}
			}

			// TODO think about potentially infinite session in aaa
			err = engine.AddPresence(e.Channel, client.UserID(), clientInfo, presenceDuration)
			if err != nil {
				Logger.Errorf("Error during AddPresence %v", err)
			}
			Logger.Infof("Added presence for userId %v", client.UserID())
			getChanPresenceStats(engine, client, e)

			return centrifuge.SubscribeReply{}
		})

		client.On().Unsubscribe(func(e centrifuge.UnsubscribeEvent) centrifuge.UnsubscribeReply {
			err := engine.RemovePresence(e.Channel, client.UserID())
			if err != nil {
				Logger.Errorf("Error during RemovePresence %v", err)
			}
			Logger.Infof("Removed presence for userId %v", client.UserID())
			getChanPresenceStats(engine, client, e)

			return centrifuge.UnsubscribeReply{}
		})

		// Set Publish Handler to react on every channel Publication sent by client.
		// Inside this method you can validate client permissions to publish into
		// channel. But in our simple chat app we allow everyone to publish into
		// any channel.
		client.On().Publish(func(e centrifuge.PublishEvent) centrifuge.PublishReply {
			Logger.Printf("client %v publishes into channel %s: %s", creds.UserID, e.Channel, string(e.Data))
			message, err := modifyMessage(e.Data, e.Info.GetUser(), e.Info.GetClient())
			if err != nil {
				Logger.Errorf("Error during modifyMessage %v", err)
				return centrifuge.PublishReply{Error: centrifuge.ErrorInternal}
			}
			return centrifuge.PublishReply{Data: message}
		})

		// Set Disconnect Handler to react on client disconnect events.
		client.On().Disconnect(func(e centrifuge.DisconnectEvent) centrifuge.DisconnectReply {
			Logger.Printf("client %v disconnected", creds.UserID)
			onlineStorage.RemoveUserOnline(userId)
			notifyAboutOnline(node, userId, false)
			return centrifuge.DisconnectReply{}
		})

		client.On().Refresh(func(event centrifuge.RefreshEvent) centrifuge.RefreshReply {
			onlineStorage.PutUserOnline(userId)
			return centrifuge.RefreshReply{}
		})

		client.On().SubRefresh(func(event centrifuge.SubRefreshEvent) centrifuge.SubRefreshReply {
			onlineStorage.PutUserOnline(userId)
			return centrifuge.SubRefreshReply{}
		})

		client.On().Message(func(event centrifuge.MessageEvent) centrifuge.MessageReply {
			var v = &TypedMessage{}

			if err := json.Unmarshal(event.Data, v); err != nil {
				Logger.Errorf("client %v sent non-parseable message - cannot extract type", creds.UserID)
				return centrifuge.MessageReply{}
			}

			if v.Type == "message_read" {
				mr := MessageRead{}
				if njson.Unmarshal(event.Data, &mr) != nil {
					Logger.Errorf("client %v sent non-parseable message - cannot unmarshall payload", creds.UserID)
					return centrifuge.MessageReply{}
				} else {
					// TODO to separated centrifuge messages handler
					Logger.Infof("Putting message read messageId=%v, chatId=%v, userId=%v", mr.MessageId, mr.ChatId, userId)
					err = markMessageAsRead(dbs, userId, mr.ChatId, mr.MessageId)
					if err != nil {
						Logger.Errorf("Error during putting message read messageId=%v, chatId=%v, userId=%v: err=%v", mr.MessageId, mr.ChatId, userId, err)
						return centrifuge.MessageReply{}
					}
				}
			} else {
				Logger.Errorf("client %v sent message with unknown type %v", creds.UserID, v.Type)
			}

			return centrifuge.MessageReply{}
		})

		// In our example transport will always be Websocket but it can also be SockJS.
		transportName := client.Transport().Name()
		// In our example clients connect with JSON protocol but it can also be Protobuf.
		transportEncoding := client.Transport().Encoding()
		Logger.Printf("client %v connected via %s (%s)", creds.UserID, transportName, transportEncoding)
	})

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			// do some work on application stop (like closing connections and files)
			Logger.Infof("Stopping centrifuge")
			return node.Shutdown(ctx)
		},
	})

	return node
}

func checkPermissions(dbs db.DB, userId string, channelId int64, channelName string) error {
	if utils.CHANNEL_PREFIX_CHAT_MESSAGES == channelName {
		if ids, err := dbs.GetParticipantIds(channelId); err != nil {
			return err
		} else {
			for _, uid := range ids {
				if fmt.Sprintf("%v", uid) == userId {
					Logger.Infof("User %v found among participants of chat %v", userId, channelId)
					return nil
				}
			}
			return errors.New(fmt.Sprintf("User %v not found among participants", userId))
		}
	}
	return errors.New(fmt.Sprintf("User %v not allowed to use unknown channel %v", userId, channelName))
}

func getChannelId(channel string) (int64, string, error) {
	if strings.HasPrefix(channel, utils.CHANNEL_PREFIX_CHAT_MESSAGES) {
		s := channel[len(utils.CHANNEL_PREFIX_CHAT_MESSAGES):]
		if parseInt64, err := utils.ParseInt64(s); err != nil {
			return 0, "", err
		} else {
			return parseInt64, utils.CHANNEL_PREFIX_CHAT_MESSAGES, nil
		}
	} else {
		return 0, "", errors.New("Subscription to unexpected channel: '" + channel + "'")
	}
}

func markMessageAsRead(db db.DB, userId, chatId, messageId int64) error {
	if participant, err := db.IsParticipant(userId, chatId); err != nil {
		Logger.Errorf("Error during checking participant")
		return err
	} else if !participant {
		Logger.Infof("User %v is not participant of chat %v, skipping", userId, chatId)
		return errors.New("Not authorized")
	}

	if err := db.AddMessageRead(messageId, userId, chatId); err != nil {
		return err
	}
	return nil
}

type UserOnlineChanged struct {
	UserId int64 `json:"userId"`
	Online bool `json:"online"`
}

func notifyAboutOnline(node *centrifuge.Node, userId int64, online bool) {
	channels, err := node.Channels()
	if err != nil {
		Logger.Errorf("Error during getting channels")
		return
	}
	for _, ch := range channels {
		_, _, err := getChannelId(ch)
		if err == nil {
			notification := dto.CentrifugeNotification{
				Payload: []UserOnlineChanged{UserOnlineChanged{
						UserId: userId,
						Online: online,
					},
				},
				EventType: "user_online_changed",
			}
			if marshalledBytes, err := json.Marshal(notification); err != nil {
				Logger.Errorf("error during marshalling user_online_changed notification: %s", err)
			} else {
				_, err := node.Publish(ch, marshalledBytes)
				if err != nil {
					Logger.Errorf("error publishing to personal channel: %s", err)
				}
			}
		}
	}
}

func periodicNotifyAboutOnline(node *centrifuge.Node, dbs db.DB, onlineStorage redis.OnlineStorage) {
	channels, err := node.Channels()
	if err != nil {
		Logger.Errorf("Error during getting channels")
		return
	}
	for _, ch := range channels {
		chatId, _, err := getChannelId(ch)
		if err == nil {
			participantIds, err := dbs.GetParticipantIds(chatId)
			if err != nil {
				Logger.Errorf("error publishing to personal channel: %s", err)
				continue
			}

			var arr []UserOnlineChanged = make([]UserOnlineChanged, 0)
			for _, participantId := range participantIds {
				online, err := onlineStorage.GetUserOnline(participantId)
				if err != nil {
					continue
				}
				arr = append(arr, UserOnlineChanged{
					UserId: participantId,
					Online: online,
				})
			}

			notification := dto.CentrifugeNotification{
				Payload:   arr,
				EventType: "user_online_changed",
			}
			for _, participantId := range participantIds {
				if marshalledBytes, err := json.Marshal(notification); err != nil {
					Logger.Errorf("error during marshalling user_online_changed notification: %s", err)
				} else {
					participantChannel := node.PersonalChannel(utils.Int64ToString(participantId))
					_, err := node.Publish(participantChannel, marshalledBytes)
					if err != nil {
						Logger.Errorf("error publishing to personal channel: %s", err)
					}
				}
			}
		}
	}
}

func ScheduleNotifications(node *centrifuge.Node, dbs db.DB, onlineStorage redis.OnlineStorage) *chan struct{} {
	ticker := time.NewTicker(viper.GetDuration("online.notification.period"))
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <- ticker.C:
				Logger.Info("Invoked chat user online periodic notificator")
				periodicNotifyAboutOnline(node, dbs, onlineStorage)
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()
	return &quit
}

