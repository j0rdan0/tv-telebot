package main

import (
	"telegram-bot/internal/bot"
	"telegram-bot/internal/rpc"
	"telegram-bot/internal/tv"
)

func main() {
	controller := tv.NewController()
	bot.Start(controller, rpc.RegisterRPC)
}
