package main

import (
	"log"
	"digitalocean-tgbot/internal/config"
	"digitalocean-tgbot/internal/db"
	"digitalocean-tgbot/internal/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Load()
	database := db.Init("bot.db")

	tgbot, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		log.Panic(err)
	}

	tgbot.Debug = false
	log.Printf("Authorized on account %s", tgbot.Self.UserName)

	h := bot.NewHandler(tgbot, database)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := tgbot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil && update.CallbackQuery == nil {
			continue
		}

		var userID int64
		if update.Message != nil {
			userID = update.Message.From.ID
		} else {
			userID = update.CallbackQuery.From.ID
		}

		if !cfg.IsAdmin(userID) {
			continue
		}

		go func(upd tgbotapi.Update) {
			if upd.Message != nil {
				if upd.Message.IsCommand() {
					h.HandleCommand(upd.Message)
				} else {
					h.ProcessMessage(upd.Message)
				}
			} else if upd.CallbackQuery != nil {
				h.HandleCallback(upd.CallbackQuery)
			}
		}(update)
	}
}
