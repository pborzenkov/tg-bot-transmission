package bot

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/tucnak/telebot.v2"
)

// Transmission defines an interface that transmission client must implement in
// order to be usable by this bot.
type Transmission interface {
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

		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("telebot.NewBot: %v", err)
	}

	return &Bot{
		log: conf.Log,

		bot:   tb,
		trans: transmission,
	}, nil
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
