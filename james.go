package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	_ "github.com/mattn/go-sqlite3"

	"github.com/op/go-logging"
)

//logger
var log = logging.MustGetLogger("james")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

type StateStore struct{}

var _ crypto.StateStore = &StateStore{}

func (fss *StateStore) IsEncrypted(roomID id.RoomID) bool {
	return false
}

func EncryptionIsNeeded(cli *mautrix.Client, room_id id.RoomID) bool {
	var content event.EncryptionEventContent
	err := cli.StateEvent(room_id, event.StateEncryption, "", &content)
	if err != nil {
		return false
	}
	return true
}

func (fss *StateStore) GetEncryptionEvent(roomID id.RoomID) *event.EncryptionEventContent {
	return &event.EncryptionEventContent{
		Algorithm:              id.AlgorithmMegolmV1,
		RotationPeriodMillis:   7 * 24 * 60 * 60 * 1000,
		RotationPeriodMessages: 100,
	}
}

func (fss *StateStore) FindSharedRooms(userID id.UserID) []id.RoomID {
	return []id.RoomID{}
}

type Logger struct{}

var _ crypto.Logger = &Logger{}

func (f Logger) Error(message string, args ...interface{}) {
	log.Error(message, args)
}

func (f Logger) Warn(message string, args ...interface{}) {
	log.Warning(message, args)
}

func (f Logger) Debug(message string, args ...interface{}) {
	log.Notice(message, args)
}

func (f Logger) Trace(message string, args ...interface{}) {
	if strings.HasPrefix(message, "Got membership state event") {
		return
	}
	log.Info(message, args)
}

func getUserIDs(cli *mautrix.Client, roomID id.RoomID) []id.UserID {
	members, err := cli.JoinedMembers(roomID)
	if err != nil {
		panic(err)
	}
	userIDs := make([]id.UserID, len(members.Joined))
	i := 0
	for userID := range members.Joined {
		userIDs[i] = userID
		i++
	}
	return userIDs
}

func sendEncrypted(mach *crypto.OlmMachine, cli *mautrix.Client, roomID id.RoomID, text string) {
	content := event.MessageEventContent{
		MsgType: "m.text",
		Body:    text,
	}
	encrypted, err := mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	if err == crypto.SessionExpired || err == crypto.SessionNotShared || err == crypto.NoGroupSession {
		err = mach.ShareGroupSession(roomID, getUserIDs(cli, roomID))
		if err != nil {
			panic(err)
		}
		encrypted, err = mach.EncryptMegolmEvent(roomID, event.EventMessage, content)
	}
	if err != nil {
		panic(err)
	}
	_, err = cli.SendMessageEvent(roomID, event.EventEncrypted, encrypted)
	if err != nil {
		panic(err)
	}
}

type Config struct {
	Homeserver   string `json:"homeserver"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Client_name  string `json:"client_name"`
	Welcome_text string `json:"welcome_text"`
	Commander    string `json:"commander"`
	LogLevel     string `json:"loglevel"`
}

func load_config(file string) (Config, error) {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		log.Error(err)
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	return config, err
}

func main() {
	start := time.Now().UnixNano() / 1_000_000

	//Logger
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backend)

	logging.SetBackend(backendLeveled, backendFormatter)
	config, err := load_config("conf.json")
	if err != nil {
		panic(err)
	} else {
		level, err := logging.LogLevel(config.LogLevel)
		if err != nil {
			logging.SetLevel(logging.ERROR, "")
		}
		logging.SetLevel(level, "")
		log.Info("Config file loaded succesfully")
	}
	log.Info("Logging into", config.Homeserver, "as", config.Username)

	cli, err := mautrix.NewClient(config.Homeserver, "", "")
	if err != nil {
		panic(err)
	}
	_, err = cli.Login(&mautrix.ReqLogin{
		Type:                     "m.login.password",
		Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: config.Username},
		Password:                 config.Password,
		InitialDeviceDisplayName: config.Client_name,
		StoreCredentials:         true,
	})

	if err != nil {
		panic(err)
	}
	log.Info("Login successful")
	myname, err := cli.GetOwnDisplayName()
	if err != nil {
		panic(err)
	}
	defer func() {

		log.Info("Logging out")
		_, err := cli.Logout()
		if err != nil {
			log.Error(err)
		}
	}()

	log.Info("Init crypto module")
	db, err := sql.Open("sqlite3", "mydb.db")
	if err != nil {
		panic(err)
	}
	logger := Logger{}
	cryptoStore := crypto.NewSQLCryptoStore(db, "sqlite3", cli.DeviceID.String(), cli.DeviceID, []byte(cli.DeviceID.String()+"bob"), &logger)
	if err != nil {
		panic(err)
	}
	if err = cryptoStore.CreateTables(); err != nil {
		panic(err)
	}
	mach := crypto.NewOlmMachine(cli, &Logger{}, cryptoStore, &StateStore{})
	err = mach.Load()
	if err != nil {
		panic(err)
	}

	log.Info("Crypto module init succesful")
	var resp *mautrix.RespJoinedRooms
	resp, err = cli.JoinedRooms()
	if err != nil {
		panic(err)
	}
	log.Info("Send welcome message to joined rooms")
	if resp != nil {
		for _, room := range resp.JoinedRooms {
			if EncryptionIsNeeded(cli, room) {
				go sendEncrypted(mach, cli, room, config.Welcome_text)
			} else {
				_, err = cli.SendText(room, config.Welcome_text)
				if err != nil {
					log.Error(err)
				}
			}

		}
	}
	fmt.Println("Bot started")
	syncer := cli.Syncer.(*mautrix.DefaultSyncer)
	syncer.OnSync(func(resp *mautrix.RespSync, since string) bool {
		mach.ProcessSyncResponse(resp, since)
		return true
	})
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, ev *event.Event) {
		mach.HandleMemberEvent(ev)
	})
	// Listen to encrypted messages
	syncer.OnEventType(event.EventEncrypted, func(source mautrix.EventSource, ev *event.Event) {
		if ev.Timestamp < start {
			// Ignore events from before the program started
			return
		}
		decrypted, err := mach.DecryptMegolmEvent(ev)
		if err != nil {
			log.Error("Failed to decrypt:")
			log.Error(err)
		} else {
			log.Info("Received encrypted event:", decrypted.Content.Raw)
			message, _ := decrypted.Content.Parsed.(*event.MessageEventContent)
			log.Info("R: ", decrypted.RoomID, decrypted.Sender, ": ", message.Body)

			commandhandler(mach, cli, message.Body, decrypted.RoomID, myname.DisplayName)

		}
	})

	syncer.OnEventType(event.EventMessage, func(_ mautrix.EventSource, ev *event.Event) {
		if ev.Timestamp < start {
			// Ignore events from before the program started
			return
		}
		log.Info("R: ", ev.RoomID, ev.Sender, ": ", ev.Content.AsMessage().Body)
		if string(ev.Sender) == config.Commander {
			commandhandler(mach, cli, ev.Content.AsMessage().Body, ev.RoomID, myname.DisplayName)
		}

	})

	syncer.OnEventType(event.StateMember, func(_ mautrix.EventSource, ev *event.Event) {
		if ev.Timestamp < start {
			// Ignore events from before the program started
			return
		}
		if ev.Content.AsMember().Membership != "invite" {
			return
		}
		if ev.Content.AsMember().Membership == "invite" {
			_, err := cli.JoinRoom(ev.RoomID.String(), "", nil)
			if err != nil {
				log.Error(err)
			}
		} else {
			log.Info("Event:", ev.Content.AsMember().Membership)
		}

	})

	go func() {
		err = cli.Sync()
		if err != nil {
			panic(err)
		}
	}()
	//logout
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "quit" {
			fmt.Println("Logout")
			break
		}
	}
}
