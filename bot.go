package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file,err: ", err)
	}

}

func NewBot() (*telego.Bot, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	bot, err := telego.NewBot(token, telego.WithDefaultDebugLogger())
	if err != nil {
		return nil, err
	}
	return bot, err

}

func startBot() {
	// Automatically start/get ngrok URL
	ngrokURL, err := StartNgrok()
	if err != nil {
		log.Printf("Warning: Failed to automate ngrok: %v. Falling back to config.", err)
		ngrokURL = LoadConfig().NgrokURL
	} else {
		// Persist the new URL
		_ = SaveNgrokURL(ngrokURL)
	}

	bot, err := NewBot()
	if err != nil {
		log.Fatal("failed initializing bot, err: ", err)
	}

	// Set webhook using dynamic URL
	webhookURL := ngrokURL + "/bot"
	err = bot.SetWebhook(context.Background(), &telego.SetWebhookParams{
		URL:         webhookURL,
		SecretToken: bot.SecretToken(),
	})
	if err != nil {
		log.Fatalf("failed to set webhook: %v", err)
	}

	mux := http.NewServeMux()

	updates, _ := bot.UpdatesViaWebhook(context.Background(), telego.WebhookHTTPServeMux(mux, "/bot", bot.SecretToken()))

	bh, _ := th.NewBotHandler(bot, updates)

	defer bh.Stop()

	// Handler for /tvstart
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVStart(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvstart"))

	// Handler for /tvstop
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVStop(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvstop"))

	// Handler for /tvnotify
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		_, _, args := tu.ParseCommandPayload(update.Message.Text)
		if args == "" {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Please provide a message. Usage: /tvnotify <message>"))
			return nil
		}
		handleTVNotify(ctx.Bot(), chatID, args)
		return nil
	}, th.CommandEqual("tvnotify"))

	// Handler for /tvmute
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		_, _, args := tu.ParseCommandPayload(update.Message.Text)

		mute := true
		if strings.ToLower(args) == "off" {
			mute = false
		} else if strings.ToLower(args) == "on" {
			mute = true
		} else if args != "" {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Usage: /tvmute [on|off]. Defaults to ON if no argument provided."))
			return nil
		}

		handleTVMute(ctx.Bot(), chatID, mute)
		return nil
	}, th.CommandEqual("tvmute"))

	// Handler for /tvvolume
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		_, _, args := tu.ParseCommandPayload(update.Message.Text)

		if args == "" {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Usage: /tvvolume <0-100>"))
			return nil
		}

		vol, err := strconv.Atoi(args)
		if err != nil || vol < 0 || vol > 100 {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Please provide a volume between 0 and 100."))
			return nil
		}

		handleTVVolume(ctx.Bot(), chatID, vol)
		return nil
	}, th.CommandEqual("tvvolume"))

	// Handler for /tvchannels
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVChannels(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvchannels"))

	// Default handler for all other messages
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)

		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Start TV").WithCallbackData("tvstart"),
				tu.InlineKeyboardButton("Stop TV").WithCallbackData("tvstop"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Mute On").WithCallbackData("tvmute_on"),
				tu.InlineKeyboardButton("Mute Off").WithCallbackData("tvmute_off"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Channels").WithCallbackData("tvchannels"),
				tu.InlineKeyboardButton("Test Notify").WithCallbackData("tvnotify_test"),
			),
		)

		message := tu.Message(chatID, "*LG TV Control Menu*\n\nSelect a command below:").
			WithReplyMarkup(keyboard).
			WithParseMode(telego.ModeMarkdownV2)

		_, err := ctx.Bot().SendMessage(context.Background(), message)
		return err
	}, th.AnyMessage())

	// Handler for button callbacks
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		query := update.CallbackQuery
		data := query.Data
		chatID := tu.ID(query.From.ID) // Fallback to user ID for chat context

		log.Printf("Received callback query: %s from user %d", data, query.From.ID)

		// Answer callback to remove loading state
		_ = ctx.Bot().AnswerCallbackQuery(context.Background(), &telego.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
		})

		switch data {
		case "tvstart":
			go handleTVStart(ctx.Bot(), chatID)
		case "tvstop":
			go handleTVStop(ctx.Bot(), chatID)
		case "tvmute_on":
			go handleTVMute(ctx.Bot(), chatID, true)
		case "tvmute_off":
			go handleTVMute(ctx.Bot(), chatID, false)
		case "tvchannels":
			go handleTVChannels(ctx.Bot(), chatID)
		case "tvnotify_test":
			go handleTVNotify(ctx.Bot(), chatID, "Telegram Bot Notification is working")
		}

		return nil
	}, th.AnyCallbackQuery())

	// Start server for receiving requests from the Telegram
	go func() {
		err := http.ListenAndServe(":8080", mux)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	bh.Start()
}

func handleTVStart(bot *telego.Bot, chatID telego.ChatID) {
	if IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already running!"))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Starting TV... please wait..."))

	tv, err := StartTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to start TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	newKey, err := tv.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	_ = tv.KeyExit()

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is ready!"))
}

func handleTVStop(bot *telego.Bot, chatID telego.ChatID) {
	if !IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already off."))
		return
	}

	tv, err := NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	newKey, err := tv.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	err = tv.Stop()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to stop TV: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV has been turned off."))
}

func handleTVNotify(bot *telego.Bot, chatID telego.ChatID, msg string) {
	if !IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	tv, err := NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	newKey, err := tv.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	err = tv.Notification(msg)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to send notification: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Notification sent!"))
}

func handleTVMute(bot *telego.Bot, chatID telego.ChatID, mute bool) {
	if !IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	tv, err := NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	_, _ = tv.Authorize(key)

	err = tv.Mute(mute)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to mute: %v", err)))
		return
	}

	status := "ON"
	if !mute {
		status = "OFF"
	}
	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Mute set to %s", status)))
}

func handleTVVolume(bot *telego.Bot, chatID telego.ChatID, vol int) {
	if !IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	tv, err := NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	_, _ = tv.Authorize(key)

	err = tv.SetVolume(vol)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to set volume: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Volume set to %d", vol)))
}

func handleTVChannels(bot *telego.Bot, chatID telego.ChatID) {
	if !IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	tv, err := NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer tv.conn.Close()

	key := os.Getenv("client_id")
	_, _ = tv.Authorize(key)

	resp, err := tv.ChannelList()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to get channel list: %v", err)))
		return
	}

	channels, ok := resp["channelList"].([]interface{})
	if !ok {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Could not parse channel list."))
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>TV Channel List</b>\n\n")

	// Limit to first 20 channels to avoid message length limits
	count := len(channels)
	if count > 20 {
		count = 20
	}

	for i := 0; i < count; i++ {
		ch := channels[i].(map[string]interface{})
		name := ch["channelName"].(string)
		number := ch["channelNumber"].(string)
		sb.WriteString(fmt.Sprintf("%s. %s\n", number, name))
	}

	if len(channels) > 20 {
		sb.WriteString(fmt.Sprintf("\n<i>...and %d more channels</i>", len(channels)-20))
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, sb.String()).WithParseMode(telego.ModeHTML))
}
