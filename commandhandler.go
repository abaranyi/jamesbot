package main

import (
	"fmt"
	"os"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/id"
)

func commandhandler(mach *crypto.OlmMachine, cli *mautrix.Client, data string, room_id id.RoomID, myname string) {
	var str = ""
	var err error
	str = strings.TrimSpace(data)
	if strings.HasPrefix(str, myname) {
		str = str[len(myname)+2:]
	} else {
		return
	}
	switch str {
	case "ping":

		if EncryptionIsNeeded(cli, room_id) {
			sendEncrypted(mach, cli, room_id, "pong")
		} else {
			_, err = cli.SendText(room_id, "pong")
			if err != nil {
				panic(err)
			}
		}
	case "logout":
		_, err = cli.Logout()
		if err != nil {
			panic(err)
		}
		fmt.Println("Logout successful")
		os.Exit(0)
	case "leave":
		log.Info("Leave event received, R: ", room_id)
		_, err = cli.LeaveRoom(room_id)
		if err != nil {
			panic(err)
		}
	}
}
