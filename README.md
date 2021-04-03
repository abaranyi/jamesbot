# jamesbot

James is a matrix bot prototype writen in go.

## Usage
- Create a user for JamesBot and set the configuration data in config file
- Accepted log levels: CRITICAL, ERROR, WARNING, NOTICE, INFO, DEBUG
- Run JamesBot with ./jamesbot
- Quit: type quit in promt

## Current Version
Version 0.1
## History
0.1 Initial version.
## Features:
- Connect into matrix server
- Send welcome message to the rooms where already joined.
- Handle all received message
- Support E2E encryption
- Automatically join the room when invite received during operation.
- Only accept commanads from preconfigured user
## Pregenerated comandhandler:
Usage:
- leave: leave the room
- logout: Logging out and stop running.
- ping: answer with pong
Example:
botname:ping
## Add extra features:
- Edit the commandhandler.go file
## Requirements:
- libolm-dev
## This project is based on tulir/mautrix-go and it licensed under the Mozilla Public License 2.0
