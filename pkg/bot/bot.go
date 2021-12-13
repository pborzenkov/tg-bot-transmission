package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
)

const (
	callbackIDLen = 36
)

// Telegram defines an interface that telegram client must implement in order
// to be usable by this bot.
type Telegram interface {
	MakeRequest(string, url.Values) (tgbotapi.APIResponse, error)
	GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error)
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	GetFileDirectURL(string) (string, error)
	AnswerCallbackQuery(tgbotapi.CallbackConfig) (tgbotapi.APIResponse, error)
}

// Transmission defines an interface that transmission client must implement in
// order to be usable by this bot.
type Transmission interface {
	AddTorrent(context.Context, *transmission.AddTorrentReq) (*transmission.NewTorrent, error)
	IsPortOpen(context.Context) (bool, error)
	GetSessionStats(context.Context) (*transmission.SessionStats, error)
	GetSession(context.Context, ...transmission.SessionField) (*transmission.Session, error)
	SetSession(context.Context, *transmission.SetSessionReq) error
	StartTorrents(context.Context, transmission.Identifier) error
	StopTorrents(context.Context, transmission.Identifier) error
	GetTorrents(context.Context, transmission.Identifier, ...transmission.TorrentField) ([]*transmission.Torrent, error)
	RemoveTorrents(context.Context, transmission.Identifier, bool) error
}

// Bot implement transmission telegram bot.
type Bot struct {
	log Logger

	tg    Telegram
	trans Transmission
	http  *http.Client

	admins map[string]struct{}

	commands          map[string]*botCommand
	shouldSetCommands bool

	locations      map[string]string
	locationsOrder []string

	newID     func() string
	mu        sync.Mutex
	callbacks map[string]callbackHandler
}

type botCommand struct {
	description string
	handler     func(context.Context, *tgbotapi.Message, string) (tgbotapi.Chattable, error)
	dontSet     bool
}

type callbackHandler struct {
	tmr *time.Timer
	fn  callbackHandlerFn
}

type callbackHandlerFn func(ctx context.Context, q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error)

// New returns new instance of the Bot with the given token that talks to
// Transmission client using transmission.
func New(tg Telegram, transmission Transmission, opts ...Option) *Bot {
	conf := defaultConfig()
	for _, opt := range opts {
		opt.apply(conf)
	}

	b := &Bot{
		log: conf.Log,

		tg:                tg,
		trans:             transmission,
		http:              conf.HTTPClient,
		admins:            make(map[string]struct{}),
		shouldSetCommands: conf.SetCommands,

		locations:      make(map[string]string, len(conf.Locations)),
		locationsOrder: make([]string, 0, len(conf.Locations)),

		newID:     conf.NewCallbackID,
		callbacks: make(map[string]callbackHandler),
	}
	for _, u := range conf.AllowedUsers {
		b.admins[u] = struct{}{}
	}
	for _, l := range conf.Locations {
		b.locations[l.Name] = l.Path
		b.locationsOrder = append(b.locationsOrder, l.Name)
	}

	b.commands = map[string]*botCommand{
		"start": {
			dontSet: true,
			handler: func(_ context.Context, m *tgbotapi.Message, _ string) (tgbotapi.Chattable, error) {
				return reply(m, withText("Drop me a magnet link/torrent URL or a torrent file.")), nil
			},
		},
		"checkport": {
			description: "Check if the incoming port is open",
			handler: func(ctx context.Context, m *tgbotapi.Message, _ string) (tgbotapi.Chattable, error) {
				return b.checkPort(ctx, m)
			},
		},
		"stats": {
			description: "Show session statistics",
			handler: func(ctx context.Context, m *tgbotapi.Message, _ string) (tgbotapi.Chattable, error) {
				return b.stats(ctx, m)
			},
		},
		"turtleon": {
			description: "Enable turtle mode",
			handler: func(ctx context.Context, m *tgbotapi.Message, _ string) (tgbotapi.Chattable, error) {
				return b.setTurtle(ctx, m, true)
			},
		},
		"turtleoff": {
			description: "Disable turtle mode",
			handler: func(ctx context.Context, m *tgbotapi.Message, _ string) (tgbotapi.Chattable, error) {
				return b.setTurtle(ctx, m, false)
			},
		},
		"resume": {
			description: "Resume specified torrents",
			handler:     b.resumeTorrents,
		},
		"stop": {
			description: "Stop specified torrents",
			handler:     b.stopTorrents,
		},
		"list": {
			description: "List torrents",
			handler:     b.listTorrents,
		},
		"remove": {
			description: "Remove torrents",
			handler:     b.removeTorrents,
		},
	}

	return b
}

// Run runs the bot until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	if b.shouldSetCommands {
		b.setCommands(ctx)
	}

	offset := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// TODO: can't be interrupted without context support
		updates, err := b.tg.GetUpdates(tgbotapi.UpdateConfig{
			Offset:  offset,
			Timeout: 10,
		})
		if err != nil {
			b.log.Infof("can't receive updates from Telegram API: %v", err)
			// TODO: handle context
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			reply := b.processUpdate(ctx, u)
			if reply != nil {
				_, err := b.tg.Send(reply)
				if err != nil {
					b.log.Infof("failed to send reply to %d: %v", u.UpdateID, err)
				}
			}
		}
	}
}

func (b *Bot) setCommands(_ context.Context) {
	type tgBotCommand struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	var commands []tgBotCommand

	for name, c := range b.commands {
		if c.dontSet {
			continue
		}
		commands = append(commands, tgBotCommand{
			Command:     name,
			Description: c.description,
		})
	}
	data, err := json.Marshal(commands)
	if err != nil {
		b.log.Infof("failed to marshal a list of the bot commands: %v", err)
		return
	}

	b.log.Debugf("uploading a list of %d commands", len(commands))

	v := url.Values{}
	v.Add("commands", string(data))
	if _, err := b.tg.MakeRequest("setMyCommands", v); err != nil {
		b.log.Infof("failed to upload a list of the bot commands: %v", err)
	}
}

func getUser(u tgbotapi.Update) *tgbotapi.User {
	if u.Message != nil && u.Message.From != nil {
		return u.Message.From
	}
	if u.CallbackQuery != nil && u.CallbackQuery.From != nil {
		return u.CallbackQuery.From
	}

	return nil
}

func (b *Bot) processUpdate(ctx context.Context, u tgbotapi.Update) tgbotapi.Chattable {
	user := getUser(u)
	if user == nil {
		return nil
	}

	if _, ok := b.admins[user.UserName]; !ok {
		return reply(u.Message, withText("Sorry, I don't know you..."))
	}

	switch {
	case u.Message != nil && u.Message.IsCommand():
		return b.handleCommand(ctx, u.Message)
	case u.Message != nil && u.Message.Text != "":
		return b.handleText(ctx, u.Message)
	case u.Message != nil && u.Message.Document != nil:
		return b.handleDocument(ctx, u.Message)
	case u.CallbackQuery != nil:
		return b.handleCallback(ctx, u.CallbackQuery)
	default:
		return nil
	}
}

func (b *Bot) handleCommand(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	cmd, ok := b.commands[m.Command()]
	if !ok {
		return reply(m, withText("Unknown command"))
	}

	r, err := cmd.handler(ctx, m, m.CommandArguments())
	if err != nil {
		return reply(m, withError(err))
	}
	return r
}

func (b *Bot) handleText(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	r, err := b.addTorrent(ctx, m, &transmission.AddTorrentReq{
		URL: transmission.OptString(m.Text),
	})
	if err != nil {
		return reply(m, withError(err))
	}
	return r
}

func (b *Bot) handleDocument(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	furl, err := b.tg.GetFileDirectURL(m.Document.FileID)
	if err != nil {
		return reply(m, withError(err))
	}
	req, err := http.NewRequestWithContext(ctx, "GET", furl, nil)
	if err != nil {
		return reply(m, withError(err))
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return reply(m, withError(err))
	}
	defer resp.Body.Close()

	data := new(bytes.Buffer)
	if _, err := io.Copy(data, resp.Body); err != nil {
		return reply(m, withError(err))
	}

	r, err := b.addTorrent(ctx, m, &transmission.AddTorrentReq{
		Meta: data,
	})
	if err != nil {
		return reply(m, withError(err))
	}
	return r
}

func (b *Bot) addCallbackHandler(fn callbackHandlerFn) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.newID()
	b.callbacks[id] = callbackHandler{
		tmr: time.AfterFunc(time.Hour, func() {
			b.mu.Lock()
			delete(b.callbacks, id)
			b.mu.Unlock()
		}),
		fn: fn,
	}

	return id
}

func getCallbackID(cb *tgbotapi.CallbackQuery) string {
	var id string
	if len(cb.Data) > callbackIDLen {
		id = cb.Data[:callbackIDLen]
		cb.Data = cb.Data[callbackIDLen:]
	}
	return id
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) tgbotapi.Chattable {
	id := getCallbackID(cb)
	b.mu.Lock()
	handler, ok := b.callbacks[id]
	delete(b.callbacks, id)
	b.mu.Unlock()
	if _, err := b.tg.AnswerCallbackQuery(tgbotapi.NewCallback(cb.ID, "")); err != nil {
		return edit(cb.Message, withError(err))
	}
	if !ok {
		return edit(cb.Message, withText("Looks like these buttons no longer work ¯\\_(ツ)_/¯"))
	}
	handler.tmr.Stop()

	r, err := handler.fn(ctx, cb)
	if err != nil {
		return edit(cb.Message, withError(err))
	}

	return r
}
