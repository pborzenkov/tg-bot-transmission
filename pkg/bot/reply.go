package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	markdownV2Replacer = strings.NewReplacer(
		func(chars string) []string {
			out := make([]string, 0, len(chars)*2)
			for _, c := range chars {
				out = append(out, string(c), "\\"+string(c))
			}
			return out
		}("-*[]()~`>#+-=|{}.!")...,
	)
)

func escapeMarkdownV2(s string) string {
	return markdownV2Replacer.Replace(s)
}

func replyText(m *tgbotapi.Message, txt string, opts ...textOption) tgbotapi.Chattable {
	msg := tgbotapi.NewMessage(m.Chat.ID, txt)
	for _, opt := range opts {
		opt(&msg)
	}
	return msg
}

type textOption func(*tgbotapi.MessageConfig)

func withParseMode(mode string) textOption {
	return func(msg *tgbotapi.MessageConfig) {
		msg.ParseMode = mode
	}
}

func replyError(m *tgbotapi.Message, err error) tgbotapi.Chattable {
	return replyText(m, "Oops, something went wrong: "+err.Error())
}
