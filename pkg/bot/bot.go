package bot

import (
	"context"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
)

// Telegram defines an interface that telegram client must implement in order
// to be usable by this bot.
type Telegram interface {
	GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error)
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	GetFileDirectURL(string) (string, error)
}

// Transmission defines an interface that transmission client must implement in
// order to be usable by this bot.
type Transmission interface {
	AddTorrent(context.Context, *transmission.AddTorrentReq) (*transmission.NewTorrent, error)
	IsPortOpen(context.Context) (bool, error)
}

// Bot implement transmission telegram bot.
type Bot struct {
	log Logger

	tg    Telegram
	trans Transmission
	admin string
}

// New returns new instance of the Bot with the given token that talks to
// Transmission client using transmission.
func New(tg Telegram, transmission Transmission, opts ...Option) (*Bot, error) {
	conf := defaultConfig()
	for _, opt := range opts {
		opt.apply(conf)
	}

	b := &Bot{
		log: conf.Log,

		tg:    tg,
		trans: transmission,
		admin: conf.AllowedUser,
	}

	return b, nil
}

// Run runs the bot until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
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

func (b *Bot) processUpdate(ctx context.Context, u tgbotapi.Update) tgbotapi.Chattable {
	if u.Message == nil || u.Message.From == nil {
		return nil
	}

	if u.Message.From.UserName != b.admin {
		return replyText(u.Message, "Sorry, I don't know you...")
	}

	switch {
	case u.Message.IsCommand():
		return b.handleCommand(ctx, u.Message)
	case u.Message.Text != "":
		return b.handleText(ctx, u.Message)
	case u.Message.Document != nil:
		return b.handleDocument(ctx, u.Message)
	default:
		return nil
	}
}

func (b *Bot) handleCommand(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	switch m.Command() {
	case "start":
		return replyText(m, "Drop me a magnet link/torrent URL or a torrent file.")
	case "checkport":
		return b.checkPort(ctx, m)
	default:
		return replyText(m, "Unknown command")
	}
}

func (b *Bot) handleText(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	return b.addTorrent(ctx, m, &transmission.AddTorrentReq{
		URL: transmission.OptString(m.Text),
	})
}

func (b *Bot) handleDocument(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	furl, err := b.tg.GetFileDirectURL(m.Document.FileID)
	if err != nil {
		return replyError(m, err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", furl, nil)
	if err != nil {
		return replyError(m, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return replyError(m, err)
	}
	defer resp.Body.Close()

	return b.addTorrent(ctx, m, &transmission.AddTorrentReq{
		Meta: resp.Body,
	})
}
