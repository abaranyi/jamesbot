package main

import (
	"fmt"
	"os"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/id"
)

func sendMessage(mach *crypto.OlmMachine, cli *mautrix.Client, room_id id.RoomID, message string) {
	var err error
	if EncryptionIsNeeded(cli, room_id) {
		sendEncrypted(mach, cli, room_id, message)
	} else {
		_, err = cli.SendText(room_id, message)
		if err != nil {
			log.Error(err)
		}
	}
}

func commandhandler(mach *crypto.OlmMachine, cli *mautrix.Client, data string, room_id id.RoomID, myname string) {
	var str = ""
	var err error

	str = strings.TrimSpace(data)
	if strings.HasPrefix(str, myname) {
		str = str[len(myname)+2:]
	} else {
		return
	}
	log.Info("Slice the text")
	strArray := strings.Fields(str)
	log.Info(strArray[0])
	switch strArray[0] {
	case "ping":
		sendMessage(mach, cli, room_id, "pong")
	case "logout":
		_, err = cli.Logout()
		if err != nil {
			log.Critical(err)
		}
		fmt.Println("Logout successful")
		os.Exit(0)
	case "leave":
		log.Info("Leave event received, R: ", room_id)
		_, err = cli.LeaveRoom(room_id)
		if err != nil {
			log.Critical(err)
		}
	}
}
