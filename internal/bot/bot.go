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

	sharedTV   *tv.WebOSTV
	sharedTVMu sync.Mutex

	powerMu sync.Mutex
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file,err: ", err)
	}
}

func getSharedTV() (*tv.WebOSTV, error) {
	sharedTVMu.Lock()
	defer sharedTVMu.Unlock()

	if sharedTV != nil {
		return sharedTV, nil
	}

	webos, err := tv.NewWebOSTV()
	if err != nil {
		return nil, err
	}

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err != nil {
		webos.Close()
		return nil, fmt.Errorf("authorization failed: %v", err)
	}

	if newKey != "" && newKey != key {
		_ = config.SaveClientKey(newKey)
	}

	sharedTV = webos
	return sharedTV, nil
}

func clearSharedTV() {
	sharedTVMu.Lock()
	defer sharedTVMu.Unlock()
	if sharedTV != nil {
		_ = sharedTV.Close()
		sharedTV = nil
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
			{Command: "tvcurrent", Description: "Show current channel"},
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

	// Access control middleware
	bh.Use(func(ctx *th.Context, update telego.Update) error {
		cfg := config.LoadConfig()

		var userID int64
		var username string
		if update.Message != nil {
			userID = update.Message.From.ID
			username = update.Message.From.Username
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
			username = update.CallbackQuery.From.Username
		}

		// If neither restriction is set, log and allow
		if cfg.AllowedUsername == "" && cfg.AllowedUserID == 0 {
			log.Printf("SECURITY: No access control set. Allowing request from @%s (%d). Set ALLOWED_USERNAME or ALLOWED_USER_ID in .env to restrict access.", username, userID)
			return ctx.Next(update)
		}

		// Check Username first if set
		if cfg.AllowedUsername != "" {
			if username != cfg.AllowedUsername {
				log.Printf("SECURITY: Unauthorized access attempt from @%s (Expected @%s)", username, cfg.AllowedUsername)
				return nil
			}
			return ctx.Next(update)
		}

		// Fallback to User ID check
		if userID != cfg.AllowedUserID {
			log.Printf("SECURITY: Unauthorized access attempt from user %d (Expected %d)", userID, cfg.AllowedUserID)
			return nil
		}

		return ctx.Next(update)
	})

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
				tu.InlineKeyboardButton("Current Channel").WithCallbackData("tvcurrent"),
				tu.InlineKeyboardButton("Channels").WithCallbackData("tvchannels"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Set channel by number").WithCallbackData("tvsetchannel_prompt"),
				tu.InlineKeyboardButton("Set Volume").WithCallbackData("tvvolume_prompt"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("Back").WithCallbackData("tvback"),
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
		log.Printf("Handling /start from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		sendMenu(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("start"))

	// Handler for /tvstart
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		log.Printf("Handling /tvstart from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVStart(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvstart"))

	// Handler for /tvstop
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		log.Printf("Handling /tvstop from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVStop(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvstop"))

	// Handler for /tvcurrent
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		log.Printf("Handling /tvcurrent from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVCurrent(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvcurrent"))

	// Handler for /tvnotify
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		log.Printf("Handling /tvnotify from %d", update.Message.From.ID)
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
		log.Printf("Handling /tvmute from %d", update.Message.From.ID)
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
		log.Printf("Handling /tvvolume from %d", update.Message.From.ID)
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
		log.Printf("Handling /tvchannels from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVChannels(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvchannels"))

	// Handler for /tvchannel <number>
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		log.Printf("Handling /tvchannel from %d", update.Message.From.ID)
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
		log.Printf("Handling /tvback from %d", update.Message.From.ID)
		chatID := tu.ID(update.Message.Chat.ID)
		handleTVBack(ctx.Bot(), chatID)
		return nil
	}, th.CommandEqual("tvback"))

	// Numeric input handler (Direct channel switching)
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		text := update.Message.Text

		log.Printf("Handling general message: %s from %d", text, update.Message.From.ID)

		// If it's a command that fell through, don't treat it as a channel number
		if strings.HasPrefix(text, "/") {
			log.Printf("Unrecognized command: %s", text)
			sendMenu(ctx.Bot(), chatID)
			return nil
		}

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
		case "tvcurrent":
			go handleTVCurrent(ctx.Bot(), chatID)
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
	if !powerMu.TryLock() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "A power operation is already in progress. Please wait."))
		return
	}
	defer powerMu.Unlock()

	if !tv.IsRunning() {
		// TV is definitely off
		triggerWake(bot, chatID)
		return
	}

	webos, err := getSharedTV()
	if err != nil {
		// Connection failed, treat as off
		triggerWake(bot, chatID)
		return
	}

	state, _ := webos.GetPowerState()
	log.Printf("TV Power State for Start: %s", state)

	if state == "active" || state == "On" {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already running!"))
		return
	}

	// Standby or other, trigger wake
	triggerWake(bot, chatID)
}

func triggerWake(bot *telego.Bot, chatID telego.ChatID) {
	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Starting TV... please wait..."))
	webos, err := tv.StartTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to start TV: %v", err)))
		return
	}

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err == nil && newKey != key {
		_ = config.SaveClientKey(newKey)
	}

	// Save to shared TV instance
	sharedTVMu.Lock()
	if sharedTV != nil {
		sharedTV.Close()
	}
	sharedTV = webos
	sharedTVMu.Unlock()

	_ = webos.KeyExit()
	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is ready!"))
}

func handleTVStop(bot *telego.Bot, chatID telego.ChatID) {
	if !powerMu.TryLock() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "A power operation is already in progress. Please wait."))
		return
	}
	defer powerMu.Unlock()

	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already off."))
		return
	}

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already off (unreachable)."))
		return
	}

	state, _ := webos.GetPowerState()
	log.Printf("TV Power State for Stop: %s", state)

	if state == "standby" || state == "Screen Off" {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is already in standby."))
		return
	}

	err = webos.Stop()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to stop TV: %v", err)))
		clearSharedTV()
		return
	}

	clearSharedTV()
	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV has been turned off."))
}

func handleTVCurrent(bot *telego.Bot, chatID telego.ChatID) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	resp, err := webos.GetCurrentChannel()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to get current channel: %v", err)))
		clearSharedTV()
		return
	}

	name := resp["channelName"].(string)
	number := resp["channelNumber"].(string)

	msg := fmt.Sprintf("📺 <b>Current Channel</b>\n\n<b>Number:</b> %s\n<b>Name:</b> %s", number, name)
	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, msg).WithParseMode(telego.ModeHTML))
}

func handleTVNotify(bot *telego.Bot, chatID telego.ChatID, msg string) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	err = webos.Notification(msg)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to send notification: %v", err)))
		clearSharedTV()
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Notification sent!"))
}

func handleTVMute(bot *telego.Bot, chatID telego.ChatID, mute bool) {
	if !tv.IsRunning() {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
		return
	}

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	err = webos.Mute(mute)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to mute: %v", err)))
		clearSharedTV()
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

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	err = webos.SetVolume(vol)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to set volume: %v", err)))
		clearSharedTV()
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

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	resp, err := webos.ChannelList()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to get channel list: %v", err)))
		clearSharedTV()
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

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	// 1. Get current channel to save its ID for "Back"
	curr, err := webos.GetCurrentChannel()
	if err == nil {
		if chId, ok := curr["channelId"].(string); ok {
			pcMu.Lock()
			previousChannels[chatID] = chId
			pcMu.Unlock()
		}
	} else {
		clearSharedTV()
	}

	// 2. Get channel list to find the ID for the provided number
	resp, err := webos.ChannelList()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to get channel list: %v", err)))
		clearSharedTV()
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
		clearSharedTV()
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

	webos, err := getSharedTV()
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
		return
	}

	// Save current ID before going back
	curr, err := webos.GetCurrentChannel()
	if err == nil {
		if chId, ok := curr["channelId"].(string); ok {
			pcMu.Lock()
			previousChannels[chatID] = chId
			pcMu.Unlock()
		}
	} else {
		clearSharedTV()
	}

	err = webos.SetChannel(prevId)
	if err != nil {
		_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to go back: %v", err)))
		clearSharedTV()
		return
	}

	_, _ = bot.SendMessage(context.Background(), tu.Message(chatID, "Switched back to previous channel."))
}
