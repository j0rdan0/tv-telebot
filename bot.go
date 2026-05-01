package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

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

		if IsRunning() {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "TV is already running!"))
			return nil
		}

		_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Starting TV... please wait..."))

		tv, err := StartTV()
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to start TV: %v", err)))
			return err
		}
		defer tv.conn.Close()

		key := os.Getenv("client_id")
		newKey, err := tv.Authorize(key)
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
			return err
		}

		if newKey != key {
			if err := SaveClientKey(newKey); err != nil {
				log.Printf("Failed to save client key: %v", err)
			}
		}

		_ = tv.KeyExit()

		_, err = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "TV is ready!"))
		return err
	}, th.CommandEqual("tvstart"))

	// Handler for /tvstop
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)

		if !IsRunning() {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "TV is already off."))
			return nil
		}

		tv, err := NewWebOSTV()
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
			return err
		}
		defer tv.conn.Close()

		key := os.Getenv("client_id")
		newKey, err := tv.Authorize(key)
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
			return err
		}

		if newKey != key {
			if err := SaveClientKey(newKey); err != nil {
				log.Printf("Failed to save client key: %v", err)
			}
		}

		err = tv.Stop()
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to stop TV: %v", err)))
			return err
		}

		_, err = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "TV has been turned off."))
		return err
	}, th.CommandEqual("tvstop"))

	// Handler for /tvnotify
	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		chatID := tu.ID(update.Message.Chat.ID)
		_, _, args := tu.ParseCommandPayload(update.Message.Text)
		if args == "" {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Please provide a message. Usage: /tvnotify <message>"))
			return nil
		}

		if !IsRunning() {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "TV is not running."))
			return nil
		}

		tv, err := NewWebOSTV()
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to connect to TV: %v", err)))
			return err
		}
		defer tv.conn.Close()

		key := os.Getenv("client_id")
		newKey, err := tv.Authorize(key)
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Authorization failed: %v", err)))
			return err
		}

		if newKey != key {
			if err := SaveClientKey(newKey); err != nil {
				log.Printf("Failed to save client key: %v", err)
			}
		}

		err = tv.Notification(args)
		if err != nil {
			_, _ = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, fmt.Sprintf("Failed to send notification: %v", err)))
			return err
		}

		_, err = ctx.Bot().SendMessage(context.Background(), tu.Message(chatID, "Notification sent!"))
		return err
	}, th.CommandEqual("tvnotify"))

	messages := make([]string, 0)

	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if update.Message != nil {
			fmt.Println(update.Message.Text)
			messages = append(messages, update.Message.Text)

			resp := tu.Message(tu.ID(update.Message.Chat.ID), "testing")
			_, err := ctx.Bot().SendMessage(context.Background(), resp)
			return err
		}
		return nil
	}, th.AnyMessage())

	// Start server for receiving requests from the Telegram
	go func() {
		err := http.ListenAndServe(":8080", mux)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	bh.Start()
}
