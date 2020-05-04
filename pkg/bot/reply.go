package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func replyText(m *tgbotapi.Message, txt string) tgbotapi.Chattable {
	return tgbotapi.NewMessage(m.Chat.ID, txt)
}

func replyError(m *tgbotapi.Message, err error) tgbotapi.Chattable {
	return replyText(m, "Oops, something went wrong: "+err.Error())
}
