//go:generate mockgen -destination bot_mock_test.go -package bot . Telegram,Transmission
package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/golang/mock/gomock"
)

func newTestBot(t *testing.T, opts ...Option) (func(...update), *MockTelegram, *MockTransmission) {
	ctrl := gomock.NewController(t)

	tg := NewMockTelegram(ctrl)
	tr := NewMockTransmission(ctrl)
	bot := New(tg, tr, append(opts, WithAllowedUser("admin"))...)

	ctx, cancel := context.WithCancel(context.Background())
	return func(updates ...update) {
		calls := make([]*gomock.Call, 0, len(updates)+1)
		offset := 0
		for _, u := range updates {
			calls = append(calls, tg.EXPECT().GetUpdates(tgbotapi.UpdateConfig{
				Offset:  offset,
				Timeout: 10,
			}).Return([]tgbotapi.Update{u.Update}, nil))
			offset = u.UpdateID + 1
		}
		calls = append(calls, tg.EXPECT().GetUpdates(tgbotapi.UpdateConfig{
			Offset:  offset,
			Timeout: 10,
		}).DoAndReturn(func(_ tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
			cancel()
			return []tgbotapi.Update{}, nil
		}))
		gomock.InOrder(calls...)

		bot.Run(ctx)
	}, tg, tr
}

type messageMatcher struct {
	chatID int64
	msg    string
}

func message(chatID int64, msg string) *messageMatcher {
	return &messageMatcher{
		chatID: chatID,
		msg:    msg,
	}
}

func (m *messageMatcher) Matches(x interface{}) bool {
	msg, ok := x.(tgbotapi.MessageConfig)
	if !ok {
		return false
	}

	return msg.BaseChat.ChatID == m.chatID && strings.Contains(msg.Text, m.msg)
}

func (m *messageMatcher) String() string {
	return "Message is"
}

type updateGenerator struct {
	id int
}

type update struct {
	tgbotapi.Update
}

func (u *update) chatID() int64 {
	return u.Message.Chat.ID
}

func (u *updateGenerator) newMessage(opts ...func(*tgbotapi.Update)) update {
	u.id++

	upd := tgbotapi.Update{
		UpdateID: u.id,
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: 123,
			},
			From: &tgbotapi.User{
				UserName: "admin",
			},
		},
	}
	for _, opt := range opts {
		opt(&upd)
	}

	return update{upd}
}

func withUser(user string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.From.UserName = user
	}
}

func withCommand(cmd string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Text = "/" + cmd
		u.Message.Entities = &[]tgbotapi.MessageEntity{
			{
				Type:   "bot_command",
				Offset: 0,
				Length: len(cmd) + 1,
			},
		}
	}
}

func TestAuth(t *testing.T) {
	run, tg, _ := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withUser("testuser"))
	tg.EXPECT().Send(message(update.chatID(), "I don't know you"))
	run(update)
}

func TestStart(t *testing.T) {
	run, tg, _ := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("start"))

	tg.EXPECT().Send(message(update.chatID(), "Drop me"))
	run(update)
}

func TestCheckPort(t *testing.T) {
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("checkport"))

	var tests = []struct {
		name   string
		ret    bool
		expect string
	}{
		{name: "open", ret: true, expect: "open"},
		{name: "closed", ret: false, expect: "closed"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			run, tg, tr := newTestBot(t)

			tg.EXPECT().Send(message(123, tc.expect))
			tr.EXPECT().IsPortOpen(gomock.Any()).Return(tc.ret, nil)

			run(update)
		})
	}
}
