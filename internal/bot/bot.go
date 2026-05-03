package bot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"telegram-bot/internal/config"
	"telegram-bot/internal/ngrok"
	"telegram-bot/internal/tv"
)

var (
	previousChannels = make(map[telego.ChatID]string)
	pcMu             sync.Mutex
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

func Start() {
	// Automatically start/get ngrok URL
	ngrokURL, err := ngrok.StartNgrok()
	if err != nil {
		log.Printf("Warning: Failed to automate ngrok: %v. Falling back to config.", err)
		ngrokURL = config.LoadConfig().NgrokURL
	} else {
		// Persist the new URL
		_ = config.SaveNgrokURL(ngrokURL)
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

	// Register bot commands with Telegram
	_ = bot.SetMyCommands(context.Background(), &telego.SetMyCommandsParams{
		Commands: []telego.BotCommand{
			{Command: "start", Description: "Show control menu"},
			{Command: "tvstart", Description: "Start TV"},
			{Command: "tvstop", Description: "Stop TV"},
			{Command: "tvchannels", Description: "List channels"},
			{Command: "tvchannel", Description: "Set channel by number"},
			{Command: "tvback", Description: "Back to previous channel"},
			{Command: "tvmute", Description: "Toggle mute"},
			{Command: "tvvolume", Description: "Set volume (0-100)"},
			{Command: "tvnotify", Description: "Send notification"},
		},
	})

	mux := http.NewServeMux()

	updates, _ := bot.UpdatesViaWebhook(context.Background(), telego.WebhookHTTPServeMux(mux, "/bot", bot.SecretToken()))

	bh, _ := th.NewBotHandler(bot, updates)

	defer bh.Stop()

	// Menu helper function
	sendMenu := func(bot *telego.Bot, chatID telego.ChatID) {
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
				tu.InlineKeyboardButton("Set channel by number").WithCallbackData("tvsetchannel_prompt"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Set Volume").WithCallbackData("tvvolume_prompt"),
				tu.InlineKeyboardButton("Back").WithCallbackData("tvback"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Test Notify").WithCallbackData("tvnotify_test"),
			),
		)

		message := tu.Message(chatID, "<b>LG TV Control Menu</b>\n\nSelect an action below or send a channel number directly:").
			WithReplyMarkup(keyboard).
			WithParseMode(telego.ModeHTML)

		_, _ = bot.SendMessage(context.Background(), message)
	}

	// Handler for /start
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		sendMenu(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("start"))

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

	// Handler for /tvchannel <number>
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		_, _, args := tu.ParseCommandPayload(update.Message.Text)
		if args == "" {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Please provide a channel number. Usage: /tvchannel <number>"))
			return nil
		}
		handleTVSetChannel(ctx.Bot(), chatID, args)
		return nil
	}, th.CommandEqual("tvchannel"))

	// Handler for /tvback
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVBack(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvback"))

	// Numeric input handler (Direct channel switching)
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		text := update.Message.Text
		if _, err := strconv.Atoi(text); err == nil {
			handleTVSetChannel(ctx.Bot(), chatID, text)
			return nil
		}
		sendMenu(ctx.Bot(), chatID)
		return nil
	}, th.AnyMessage())

	// Handler for button callbacks
	bh.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		data := query.Data
		chatID := tu.ID(query.From.ID)

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
		case "tvsetchannel_prompt":
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "To set a channel, just send the channel number (e.g., 1)."))
		case "tvvolume_prompt":
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "To set volume, use /tvvolume <0-100> (e.g., /tvvolume 20)."))
		case "tvback":
			go handleTVBack(ctx.Bot(), chatID)
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
	if tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already running!"))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Starting TV... please wait..."))

	webos, err := tv.StartTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to start TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := config.SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	_ = webos.KeyExit()

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is ready!"))
}

func handleTVStop(bot *telego.Bot, chatID telego.ChatID) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already off."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := config.SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	err = webos.Stop()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to stop TV: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV has been turned off."))
}

func handleTVNotify(bot *telego.Bot, chatID telego.ChatID, msg string) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
		return
	}

	if newKey != key {
		if err := config.SaveClientKey(newKey); err != nil {
			log.Printf("Failed to save client key: %v", err)
		}
	}

	err = webos.Notification(msg)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to send notification: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Notification sent!"))
}

func handleTVMute(bot *telego.Bot, chatID telego.ChatID, mute bool) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	_, _ = webos.Authorize(key)

	err = webos.Mute(mute)
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
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	_, _ = webos.Authorize(key)

	err = webos.SetVolume(vol)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to set volume: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Volume set to %d", vol)))
}

func handleTVChannels(bot *telego.Bot, chatID telego.ChatID) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	cfg := config.LoadConfig()

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	_, _ = webos.Authorize(key)

	resp, err := webos.ChannelList()
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
	sb.WriteString("TV Channel List\n\n")

	// Limit to the configured count to avoid message length limits
	count := len(channels)
	if count > cfg.ChannelCount {
		count = cfg.ChannelCount
	}

	for i := 0; i < count; i++ {
		ch := channels[i].(map[string]interface{})
		name := ch["channelName"].(string)
		number := ch["channelNumber"].(string)
		sb.WriteString(fmt.Sprintf("%s. %s\n", number, name))
	}

	if len(channels) > cfg.ChannelCount {
		sb.WriteString(fmt.Sprintf("\n...and %d more channels", len(channels)-cfg.ChannelCount))
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, sb.String()).WithParseMode(telego.ModeHTML))
}

func handleTVSetChannel(bot *telego.Bot, chatID telego.ChatID, channelNumber string) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	_, _ = webos.Authorize(key)

	// 1. Get current channel to save its ID for "Back"
	curr, err := webos.GetCurrentChannel()
	if err == nil {
		if chId, ok := curr["channelId"].(string); ok {
			pcMu.Lock()
			previousChannels[chatID] = chId
			pcMu.Unlock()
		}
	}

	// 2. Get channel list to find the ID for the provided number
	resp, err := webos.ChannelList()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to get channel list: %v", err)))
		return
	}

	channels, ok := resp["channelList"].([]interface{})
	if !ok {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Could not parse channel list."))
		return
	}

	var targetId string
	for _, ch := range channels {
		c := ch.(map[string]interface{})
		if c["channelNumber"].(string) == channelNumber {
			targetId = c["channelId"].(string)
			break
		}
	}

	if targetId == "" {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Channel number %s not found in list.", channelNumber)))
		return
	}

	// 3. Set the new channel using ID
	err = webos.SetChannel(targetId)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to set channel: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Channel set to %s", channelNumber)))
}

func handleTVBack(bot *telego.Bot, chatID telego.ChatID) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	pcMu.Lock()
	prevId, ok := previousChannels[chatID]
	pcMu.Unlock()

	if !ok {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "No previous channel saved."))
		return
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}
	defer webos.Close()

	key := os.Getenv("client_id")
	_, _ = webos.Authorize(key)

	// Save current ID before going back
	curr, err := webos.GetCurrentChannel()
	if err == nil {
		if chId, ok := curr["channelId"].(string); ok {
			pcMu.Lock()
			previousChannels[chatID] = chId
			pcMu.Unlock()
		}
	}

	err = webos.SetChannel(prevId)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to go back: %v", err)))
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Switched back to previous channel."))
}
