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

type optionable interface {
	setText(string)
	setParseMode(string)
	setInlineKeyboard(tgbotapi.InlineKeyboardMarkup)
}

type repliable interface {
	setReplyMsgID(int)
}

type replyOption func(*tgbotapi.Message, optionable)

type message struct {
	tgbotapi.MessageConfig
}

func (m *message) setText(text string) {
	m.MessageConfig.Text = text
}

func (m *message) setParseMode(mode string) {
	m.MessageConfig.ParseMode = mode
}

func (m *message) setInlineKeyboard(kb tgbotapi.InlineKeyboardMarkup) {
	m.MessageConfig.ReplyMarkup = kb
}

func (m *message) setReplyMsgID(id int) {
	m.MessageConfig.ReplyToMessageID = id
}

type editMessage struct {
	tgbotapi.EditMessageTextConfig
}

func (m *editMessage) setText(text string) {
	m.EditMessageTextConfig.Text = text
}

func (m *editMessage) setParseMode(mode string) {
	m.EditMessageTextConfig.ParseMode = mode
}

func (m *editMessage) setInlineKeyboard(kb tgbotapi.InlineKeyboardMarkup) {
	m.ReplyMarkup = &kb
}

func reply(m *tgbotapi.Message, opts ...replyOption) tgbotapi.Chattable {
	msg := message{
		MessageConfig: tgbotapi.NewMessage(m.Chat.ID, ""),
	}
	for _, opt := range opts {
		opt(m, &msg)
	}
	return msg.MessageConfig
}

func edit(m *tgbotapi.Message, opts ...replyOption) tgbotapi.Chattable {
	edt := editMessage{
		EditMessageTextConfig: tgbotapi.NewEditMessageText(m.Chat.ID, m.MessageID, ""),
	}
	for _, opt := range opts {
		opt(m, &edt)
	}
	return edt.EditMessageTextConfig
}

func withText(text string) replyOption {
	return func(_ *tgbotapi.Message, msg optionable) {
		msg.setText(text)
	}
}

func withError(err error) replyOption {
	return withText("Oops, something went wrong: " + err.Error())
}

func withMarkdownV2() replyOption {
	return func(_ *tgbotapi.Message, msg optionable) {
		msg.setParseMode("MarkdownV2")
	}
}

func withQuoteMessage() replyOption {
	return func(m *tgbotapi.Message, msg optionable) {
		if r, ok := msg.(repliable); ok {
			r.setReplyMsgID(m.MessageID)
		}
	}
}

func withInlineKeyboard(rows ...[]tgbotapi.InlineKeyboardButton) replyOption {
	return func(_ *tgbotapi.Message, msg optionable) {
		msg.setInlineKeyboard(tgbotapi.NewInlineKeyboardMarkup(rows...))
	}
}
