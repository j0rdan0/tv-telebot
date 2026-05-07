package main

import (
	"telegram-bot/internal/bot"
	"telegram-bot/internal/rpc"
	"telegram-bot/internal/tv"
)

func main() {
	controller := tv.NewController()
	rpc.StartServer(controller, "9090")
	bot.Start(controller)
}
