//go:generate mockgen -destination bot_mock_test.go -package bot . Telegram,Transmission
package bot

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"

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

func messageMatcher(chatID int64, re string) *customMatcher {
	return &customMatcher{
		name: fmt.Sprintf("Message to %d matches re %q", chatID, re),
		matches: func(x interface{}) bool {
			msg, ok := x.(tgbotapi.MessageConfig)
			if !ok {
				return false
			}
			r := regexp.MustCompile(re)

			return msg.BaseChat.ChatID == chatID && r.MatchString(msg.Text)
		},
	}
}

func torrentMatcher(content []byte) *customMatcher {
	return &customMatcher{
		name: fmt.Sprintf("Torrent file is %q", string(content)),
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

func withText(text string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Text = text
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

func withDocument(id string) func(u *tgbotapi.Update) {
	return func(u *tgbotapi.Update) {
		u.Message.Document = &tgbotapi.Document{
			FileID: id,
		}
	}
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

	update := gen.newMessage(withText("magnet:/"))

	addTorrentCall := tr.EXPECT().AddTorrent(gomock.AssignableToTypeOf(ctxType), &transmission.AddTorrentReq{
		URL: transmission.OptString("magnet:/"),
	}).Return(&transmission.NewTorrent{
		ID:   transmission.ID(1),
		Hash: transmission.Hash("abc"),
		Name: "new fancy torrent",
	}, nil)
	tg.EXPECT().Send(messageMatcher(update.chatID(), "1.*new fancy torrent")).After(addTorrentCall)

	run(update)
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
	tg.EXPECT().Send(messageMatcher(update.chatID(), "1.*new fancy torrent")).After(getFileCall).After(addTorrentCall)

	run(update)
}

func TestStats(t *testing.T) {
	run, tg, tr := newTestBot(t)
	gen := new(updateGenerator)

	update := gen.newMessage(withCommand("stats"))

	tr.EXPECT().GetSessionStats(gomock.AssignableToTypeOf(ctxType)).Return(&transmission.SessionStats{
		DownloadRate:   2097152,
		UploadRate:     1048576,
		Torrents:       10,
		ActiveTorrents: 3,
		AllSessions: transmission.Stats{
			Downloaded: 1073741824,
			Uploaded:   2415919104,
		},
	}, nil)
	tr.EXPECT().GetSession(gomock.AssignableToTypeOf(ctxType)).Return(&transmission.Session{
		TurtleEnabled: false,
	}, nil)
	tg.EXPECT().Send(
		messageMatcher(update.chatID(), `(?s)Rate.*↓.*2\\.0 MiB/s.*↑.*1\\.0 MiB/s.*~🐢~.*`+
			`Torrents.*10.*Active.*3.*`+
			`Total.*↓.*1\\.0 GiB.*↑.*2\\.3 GiB.*☯ 2\\.25`,
		))

	run(update)
}
