package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/pborzenkov/go-transmission/transmission"

	"gopkg.in/tucnak/telebot.v2"
)

// Transmission defines an interface that transmission client must implement in
// order to be usable by this bot.
type Transmission interface {
	AddTorrent(context.Context, *transmission.AddTorrentReq) (*transmission.NewTorrent, error)
}

// Bot implement transmission telegram bot.
type Bot struct {
	log Logger

	bot   *telebot.Bot
	trans Transmission
}

// New returns new instance of the Bot with the given token that talks to
// Transmission client using transmission.
func New(token string, transmission Transmission, opts ...Option) (*Bot, error) {
	conf := defaultConfig()
	for _, opt := range opts {
		opt.apply(conf)
	}

	tb, err := telebot.NewBot(telebot.Settings{
		Token: token,

		Poller: newAuthMiddleware(conf.Log, &telebot.LongPoller{Timeout: 10 * time.Second}, conf.AllowedUsers),
	})
	if err != nil {
		return nil, fmt.Errorf("telebot.NewBot: %v", err)
	}

	b := &Bot{
		log: conf.Log,

		bot:   tb,
		trans: transmission,
	}

	b.setupHandlers()

	return b, nil
}

// Run runs the bot until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		b.bot.Start()
		b.log.Infof("done finished")
		close(done)
	}()

	<-ctx.Done()
	b.log.Infof("calling stop")
	b.bot.Stop()

	<-done
}

func newAuthMiddleware(log Logger, p telebot.Poller, allowed []string) *telebot.MiddlewarePoller {
	return telebot.NewMiddlewarePoller(p, func(u *telebot.Update) bool {
		if u.Message == nil || !u.Message.Private() {
			return true
		}
		for _, user := range allowed {
			if user == u.Message.Sender.Username {
				return true
			}
		}
		log.Debugf("auth: ignoring message from unknown user %q", u.Message.Sender.Username)

		return false
	})
}

func (b *Bot) setupHandlers() {
	b.bot.Handle(telebot.OnDocument, b.processTorrent)
	b.bot.Handle(telebot.OnText, b.processMagnet)
}
