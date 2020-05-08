package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
	"github.com/pborzenkov/tg-bot-transmission/pkg/bot"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

var (
	Version string = "dev"
)

type config struct {
	APIToken        string
	AllowUser       string
	TransmissionURL string
	Verbose         bool
}

func (c *config) command() *ffcli.Command {
	fs := flag.NewFlagSet("bot", flag.ExitOnError)
	fs.StringVar(&c.APIToken, "telegram.api-token", "", "Telegram Bot API token")
	fs.StringVar(&c.AllowUser, "telegram.allow-user", "",
		"Telegram username that's allowed to control the bot")
	fs.StringVar(&c.TransmissionURL, "transmission.url", "http://localhost:9091",
		"Transmission RPC server URL")
	fs.BoolVar(&c.Verbose, "verbose", false, "Enable verbose logging")

	root := &ffcli.Command{
		Name:       "bot",
		ShortUsage: "bot [flags]",
		FlagSet:    fs,
		Options: []ff.Option{
			ff.WithEnvVarPrefix("BOT"),
		},
		Exec: c.exec,
	}

	return root
}

func main() {
	cfg := new(config)

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT)
	go func() {
		<-sig
		cancel()
		signal.Stop(sig)
	}()

	if err := cfg.command().ParseAndRun(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func (c *config) exec(ctx context.Context, args []string) error {
	log := newLogger(os.Stdout, c.Verbose)
	log.Infof("starting, version %q", Version)

	tg, err := tgbotapi.NewBotAPI(c.APIToken)
	if err != nil {
		return fmt.Errorf("tgbotapi.NewBotAPI: %v", err)
	}
	trans, err := transmission.New(c.TransmissionURL)
	if err != nil {
		return fmt.Errorf("transmission.New: %v", err)
	}
	bot.New(tg, trans,
		bot.WithLogger(log),
		bot.WithAllowedUser(c.AllowUser),
		bot.WithSetCommands(),
	).Run(ctx)

	return nil
}
