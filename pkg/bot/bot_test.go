//go:generate mockgen -destination bot_mock_test.go -package bot . Telegram,Transmission
package bot

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/golang/mock/gomock"
	"github.com/pborzenkov/go-transmission/transmission"
)

var (
	ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

func newTestBot(t *testing.T, opts ...Option) (func(...update), *MockTelegram, *MockTransmission) {
	ctrl := gomock.NewController(t)

	tg := NewMockTelegram(ctrl)
	tr := NewMockTransmission(ctrl)
	bot := New(tg, tr, append(opts, WithAllowedUsers("admin"))...)

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

type customMatcher struct {
	name    string
	matches func(x interface{}) bool
}

func (m *customMatcher) Matches(x interface{}) bool {
	return m.matches(x)
}

func (m *customMatcher) String() string {
	return m.name
}

type msgMatchOption func(*tgbotapi.MessageConfig) bool

func hasReplyMsgID(id int) msgMatchOption {
	return func(msg *tgbotapi.MessageConfig) bool {
		return msg.ReplyToMessageID == id
	}
}

func messageMatcher(chatID int64, re string, matches ...msgMatchOption) *customMatcher {
	return &customMatcher{
		name: fmt.Sprintf("Message to %d matches re '%s'", chatID, re),
		matches: func(x interface{}) bool {
			msg, ok := x.(tgbotapi.MessageConfig)
			if !ok {
				return false
			}
			r := regexp.MustCompile(re)

			if msg.BaseChat.ChatID != chatID || !r.MatchString(msg.Text) {
				return false
			}

			for _, match := range matches {
				if !match(&msg) {
					return false
				}
			}

			return true
		},
	}
}

func torrentMatcher(content []byte) *customMatcher {
	return &customMatcher{
		name: fmt.Sprintf("Torrent file is '%s'", string(content)),
		matches: func(x interface{}) bool {
			t, ok := x.(*transmission.AddTorrentReq)
			if !ok {
				return false
			}
			if t.Meta == nil {
				return false
			}
			data, _ := ioutil.ReadAll(t.Meta)
			return bytes.Equal(data, content)
		},
	}
}

func editMatcher(chatID int64, msgID int, re string) *customMatcher {
	return &customMatcher{
		name: fmt.Sprintf("Edit of the message %d to %d matches re '%s'", msgID, chatID, re),
		matches: func(x interface{}) bool {
			msg, ok := x.(tgbotapi.EditMessageTextConfig)
			if !ok {
				return false
			}
			r := regexp.MustCompile(re)

			return msg.BaseEdit.ChatID == chatID && msg.BaseEdit.MessageID == msgID && r.MatchString(msg.Text)
		},
	}
}

func inlineKeyboardMatcher(rows ...[]tgbotapi.InlineKeyboardButton) *customMatcher {
	return &customMatcher{
		name: "Message has inline keyboard with the given keys",
		matches: func(x interface{}) bool {
			var markup *tgbotapi.InlineKeyboardMarkup

			switch m := x.(type) {
			case tgbotapi.MessageConfig:
				if k, ok := m.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
					markup = &k
				}
			case tgbotapi.EditMessageTextConfig:
				markup = m.ReplyMarkup
			}
			if markup == nil {
				return false
			}
			if len(rows) != len(markup.InlineKeyboard) {
				return false
			}
			for i := 0; i < len(rows); i++ {
				if len(rows[i]) != len(markup.InlineKeyboard[i]) {
					return false
				}
				for j := 0; j < len(rows[i]); j++ {
					if rows[i][j].Text != markup.InlineKeyboard[i][j].Text {
						return false
					}
					if *rows[i][j].CallbackData != *markup.InlineKeyboard[i][j].CallbackData {
						return false
					}
				}
			}

			return true
		},
	}
}

type updateGenerator struct {
	id int
}

type update struct {
	tgbotapi.Update
}

func (u *update) chatID() int64 {
	if u.Message == nil {
		return 0
	}
	return u.Message.Chat.ID
}

func (u *update) messageID() int {
	if u.Message == nil {
		return 0
	}
	return u.Message.MessageID
}

func (u *update) callbackID() string {
	if u.CallbackQuery == nil {
		return ""
	}
	return u.CallbackQuery.ID
}

func (u *updateGenerator) newMessage(opts ...func(*tgbotapi.Update)) update {
	u.id++

	upd := tgbotapi.Update{
		UpdateID: u.id,
		Message: &tgbotapi.Message{
			MessageID: rand.Int(), //nolint:gosec
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

func (u *updateGenerator) newCallback(msg *tgbotapi.Message, data string, opts ...func(*tgbotapi.Update)) update {
	u.id++

	upd := tgbotapi.Update{
		UpdateID: u.id,
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   strconv.Itoa(rand.Int()), //nolint:gosec
			Data: data,
			From: &tgbotapi.User{
				UserName: "admin",
			},
			Message: msg,
		},
	}
	for _, opt := range opts {
		opt(&upd)
	}

	return update{upd}
}

func withUser(user string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		switch {
		case u.Message != nil:
			u.Message.From.UserName = user
		case u.CallbackQuery != nil:
			u.CallbackQuery.From.UserName = user
		}
	}
}

func withMsgText(text string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Text = text
	}
}

func withCommand(cmd string, args ...string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Text = "/" + cmd + " " + strings.Join(args, " ")
		u.Message.Entities = &[]tgbotapi.MessageEntity{
			{
				Type:   "bot_command",
				Offset: 0,
				Length: len(cmd) + 1,
			},
		}
	}
}

func withDocument(id string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Document = &tgbotapi.Document{
			FileID: id,
		}
	}
}

func TestSetCommands(t *testing.T) {
	run, tg, _ := newTestBot(t, WithSetCommands())

	tg.EXPECT().MakeRequest("setMyCommands", gomock.Any())
	run()
}

func TestAuth(t *testing.T) {
	run, tg, _ := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withUser("testuser"))
	tg.EXPECT().Send(messageMatcher(update.chatID(), "I don't know you"))
	run(update)
}

func TestCommand_unknown(t *testing.T) {
	run, tg, _ := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("invalid"))

	tg.EXPECT().Send(messageMatcher(update.chatID(), "Unknown command"))
	run(update)
}

func TestCommand_start(t *testing.T) {
	run, tg, _ := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("start"))

	tg.EXPECT().Send(messageMatcher(update.chatID(), "Drop me"))
	run(update)
}

func TestCommand_checkPort(t *testing.T) {
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

			isPortOpenCall := tr.EXPECT().IsPortOpen(gomock.AssignableToTypeOf(ctxType)).Return(tc.ret, nil)
			tg.EXPECT().Send(messageMatcher(update.chatID(), tc.expect)).After(isPortOpenCall)
			run(update)
		})
	}
}

func TestAddTorrent_text(t *testing.T) {
	run, tg, tr := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withMsgText("magnet:/"))

	addTorrentCall := tr.EXPECT().AddTorrent(gomock.AssignableToTypeOf(ctxType), &transmission.AddTorrentReq{
		URL: transmission.OptString("magnet:/"),
	}).Return(&transmission.NewTorrent{
		ID:   transmission.ID(1),
		Hash: transmission.Hash("abc"),
		Name: "new fancy torrent",
	}, nil)
	tg.EXPECT().Send(messageMatcher(
		update.chatID(),
		`\\<\*1\*\\> new fancy torrent`,
		hasReplyMsgID(update.messageID()),
	)).After(addTorrentCall)

	run(update)
}

func TestAddTorrent_locations(t *testing.T) {
	cbID := strings.Repeat("0", callbackIDLen)
	run, tg, tr := newTestBot(t,
		WithLocations(
			Location{Name: "loc1", Path: "/path/to/loc1"},
			Location{Name: "loc2", Path: "/path/to/loc2"},
		),
		withCallbackIDGenerator(func() string { return cbID }),
	)

	gen := new(updateGenerator)

	msg := gen.newMessage(withMsgText("magnet:/"))
	cb := gen.newCallback(msg.Message, cbID+"loc1")
	updates := []update{msg, cb}

	askCall := tg.EXPECT().Send(gomock.All(
		messageMatcher(msg.chatID(), `^(?s)Ok, gonna queue it for download`),
		inlineKeyboardMatcher(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("loc1", cbID+"loc1"),
				tgbotapi.NewInlineKeyboardButtonData("loc2", cbID+"loc2"),
				tgbotapi.NewInlineKeyboardButtonData("Other", cbID+"other"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Cancel", cbID+"cancel"),
			),
		),
	))
	tg.EXPECT().AnswerCallbackQuery(tgbotapi.NewCallback(cb.callbackID(), "")).After(askCall)
	addCall := tr.EXPECT().AddTorrent(gomock.AssignableToTypeOf(ctxType), &transmission.AddTorrentReq{
		URL:               transmission.OptString("magnet:/"),
		DownloadDirectory: transmission.OptString("/path/to/loc1"),
	}).Return(&transmission.NewTorrent{
		ID:   transmission.ID(1),
		Hash: transmission.Hash("abc"),
		Name: "new fancy torrent",
	}, nil).After(askCall)
	tg.EXPECT().Send(editMatcher(msg.chatID(), msg.messageID(),
		`(?s)\\<\*1\*\\> new fancy torrent.*/path/to/loc1`)).After(addCall)

	run(updates...)
}

func TestAddTorrent_file(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want, got := "/files/file_id", r.URL.Path; want != got {
			t.Errorf("unexpected file download path, want = %q, got = %q", want, got)
		}
		if want, got := "GET", r.Method; want != got {
			t.Errorf("unexpected HTTP method, want = %q, got = %q", want, got)
		}

		if _, err := w.Write([]byte("torrent-file-contents")); err != nil {
			t.Errorf("unexpected error sending the file: %v", err)
		}
	}))
	defer srv.Close()

	run, tg, tr := newTestBot(t, WithHTTPClient(srv.Client()))
	gen := new(updateGenerator)

	update := gen.newMessage(withDocument("file_id"))

	getFileCall := tg.EXPECT().GetFileDirectURL("file_id").Return(srv.URL+"/files/file_id", nil)
	addTorrentCall := tr.EXPECT().AddTorrent(
		gomock.AssignableToTypeOf(ctxType),
		torrentMatcher([]byte("torrent-file-contents")),
	).Return(&transmission.NewTorrent{
		ID:   transmission.ID(1),
		Hash: transmission.Hash("abc"),
		Name: "new fancy torrent",
	}, nil).After(getFileCall)
	tg.EXPECT().Send(messageMatcher(
		update.chatID(),
		`\\<\*1\*\\> new fancy torrent`,
		hasReplyMsgID(update.messageID()),
	)).After(getFileCall).After(addTorrentCall)

	run(update)
}

func TestStats(t *testing.T) {
	run, tg, tr := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("stats"))

	tr.EXPECT().GetSessionStats(gomock.AssignableToTypeOf(ctxType)).Return(&transmission.SessionStats{
		DownloadRate:   2097152,
		UploadRate:     1048576,
		PausedTorrents: 10,
		ActiveTorrents: 3,
		AllSessions: transmission.Stats{
			Downloaded: 1073741824,
			Uploaded:   2415919104,
		},
	}, nil)
	tr.EXPECT().GetSession(gomock.AssignableToTypeOf(ctxType), transmission.SessionFieldTurtleEnabled).
		Return(&transmission.Session{
			TurtleEnabled: false,
		}, nil)
	tg.EXPECT().Send(
		messageMatcher(update.chatID(), `^↓\*2\\\.0 MiB/s\* ↑\*1\\\.0 MiB/s\* 🚀   `+
			`↻\*3\* ⊗\*10\*   ↓\*1\\\.0 GiB\* ↑\*2\\\.3 GiB\* ☯\*2\\\.25\*$`,
		))

	run(update)
}

func TestTurtle(t *testing.T) {
	var tests = []struct {
		name    string
		command string
		set     bool
		expect  string
	}{
		{name: "on", command: "turtleon", set: true, expect: "enabled"},
		{name: "off", command: "turtleoff", set: false, expect: "disabled"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			run, tg, tr := newTestBot(t)
			gen := new(updateGenerator)

			update := gen.newMessage(withCommand(tc.command))

			turtleCall := tr.EXPECT().SetSession(gomock.AssignableToTypeOf(ctxType), &transmission.SetSessionReq{
				TurtleEnabled: transmission.OptBool(tc.set),
			}).Return(nil)
			tg.EXPECT().Send(messageMatcher(update.chatID(), tc.expect)).After(turtleCall)
			run(update)
		})
	}
}

func TestStartStopTorrents(t *testing.T) {
	var tests = []struct {
		name      string
		command   string
		args      string
		isStart   bool
		expectIDs transmission.Identifier
	}{
		{
			name:    "resume_all",
			command: "resume",
			isStart: true,
		},
		{
			name:      "resume_two",
			command:   "resume",
			args:      "2   7",
			isStart:   true,
			expectIDs: transmission.IDs(transmission.ID(2), transmission.ID(7)),
		},
		{
			name:      "pause_one",
			command:   "stop",
			args:      "3",
			expectIDs: transmission.IDs(transmission.ID(3)),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			run, tg, tr := newTestBot(t)
			gen := new(updateGenerator)

			update := gen.newMessage(withCommand(tc.command, tc.args))

			call := tr.EXPECT().StopTorrents
			if tc.isStart {
				call = tr.EXPECT().StartTorrents
			}

			startStopCall := call(gomock.AssignableToTypeOf(ctxType), tc.expectIDs).Return(nil)
			tg.EXPECT().Send(messageMatcher(update.chatID(), "Done")).After(startStopCall)
			run(update)
		})
	}
}

func TestList(t *testing.T) {
	run, tg, tr := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("list"))

	tr.EXPECT().GetTorrents(gomock.AssignableToTypeOf(ctxType), nil, gomock.Any()).Return([]*transmission.Torrent{
		{
			ID:           1,
			Name:         "test torrent",
			Status:       transmission.StatusDownload,
			ValidSize:    1024,
			WantedSize:   2048,
			DownloadRate: 10,
			UploadRate:   20,
			UploadRatio:  1.2,
			ETA:          20 * time.Minute,
		},
	}, nil)

	tg.EXPECT().Send(
		messageMatcher(update.chatID(), `^(?s)Here is what I got:\s+`+
			`\\<\*1\*\\> \*test torrent\*`+
			`.*Downloading \*1\\\.0 KiB\* of \*2\\\.0 KiB\* \\\(\*50\\\.0%\*\\\)`+
			`.*↓\*10 B/s\* ↑\*20 B/s\* ☯\*1\\\.20\*   ETA: \*20m0s\*`),
	)

	run(update)
}

func TestList_filter(t *testing.T) {
	run, tg, tr := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("list", "second"))

	tr.EXPECT().GetTorrents(gomock.AssignableToTypeOf(ctxType), nil, gomock.Any()).Return([]*transmission.Torrent{
		{
			ID:           1,
			Name:         "first torrent",
			Status:       transmission.StatusDownload,
			ValidSize:    1024,
			WantedSize:   2048,
			DownloadRate: 10,
			UploadRate:   20,
			UploadRatio:  1.2,
			ETA:          20 * time.Minute,
		},
		{
			ID:           2,
			Name:         "Second torrent",
			Status:       transmission.StatusDownload,
			ValidSize:    1024,
			WantedSize:   2048,
			DownloadRate: 10,
			UploadRate:   20,
			UploadRatio:  1.2,
			ETA:          20 * time.Minute,
		},
	}, nil)

	tg.EXPECT().Send(messageMatcher(update.chatID(), `^(?s)Here is what I got:\s+\\<\*2\*\\> \*Second torrent\*`))

	run(update)
}

func TestRemoveTorrent(t *testing.T) {
	cbID := strings.Repeat("0", callbackIDLen)
	run, tg, tr := newTestBot(t, withCallbackIDGenerator(func() string {
		return cbID
	}))

	gen := new(updateGenerator)

	msg := gen.newMessage(withCommand("remove", "1"))
	cb := gen.newCallback(msg.Message, cbID+"yes")
	updates := []update{msg, cb}

	getCall := tr.EXPECT().GetTorrents(gomock.AssignableToTypeOf(ctxType), transmission.IDs(transmission.ID(1)),
		transmission.TorrentFieldID,
		transmission.TorrentFieldHash,
		transmission.TorrentFieldName,
	).Return([]*transmission.Torrent{
		{
			ID:   1,
			Hash: "123",
			Name: "first torrent",
		},
	}, nil)
	askCall := tg.EXPECT().Send(gomock.All(
		messageMatcher(msg.chatID(), `^(?s)I'm going to remove the following torrents:\s+`+`\\<\*1\*\\> \*first torrent\*`),
		inlineKeyboardMatcher(tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Yes", cbID+"yes"),
			tgbotapi.NewInlineKeyboardButtonData("No", cbID+"no"),
			tgbotapi.NewInlineKeyboardButtonData("Cancel", cbID+"cancel")),
		),
	)).After(getCall)
	tg.EXPECT().AnswerCallbackQuery(tgbotapi.NewCallback(cb.callbackID(), "")).After(askCall)
	removeCall := tr.EXPECT().RemoveTorrents(gomock.AssignableToTypeOf(ctxType),
		transmission.IDs(transmission.Hash("123")), true).After(askCall)
	tg.EXPECT().Send(editMatcher(msg.chatID(), msg.messageID(), "Done")).After(removeCall)

	run(updates...)
}
