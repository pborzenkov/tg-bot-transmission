package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pborzenkov/go-transmission/transmission"
	"github.com/pborzenkov/tg-bot-transmission/pkg/bot"
)

var (
	Version string = "dev"
)

type config struct {
	TelegramAPIToken  string
	TelegramAllowUser string
	TransmissionURL   string

	Verbose bool
}

func parseArgs(progname string, args []string) (*config, string, error) {
	buf := new(bytes.Buffer)
	flags := flag.NewFlagSet(progname, flag.ContinueOnError)
	flags.SetOutput(buf)

	var conf config
	flags.StringVar(&conf.TelegramAPIToken, "telegram.api-token", "", "Telegram Bot API token (required)")
	flags.StringVar(&conf.TelegramAllowUser, "telegram.allow-user", "",
		"Telegram username that's allowed to control the bot")
	flags.StringVar(&conf.TransmissionURL, "transmission.url", "http://localhost:9091", "Transmission RPC server URL")
	flags.BoolVar(&conf.Verbose, "verbose", false, "Enable verbose logging")

	if err := flags.Parse(args); err != nil {
		return nil, buf.String(), err
	}

	return &conf, buf.String(), nil
}

func main() {
	conf, out, err := parseArgs(os.Args[0], os.Args[1:])
	if err != nil {
		fmt.Println(out)
		os.Exit(1)
	}

	log := newLogger(os.Stdout, conf.Verbose)
	log.Infof("starting, version %q", Version)

	trans, err := transmission.New(conf.TransmissionURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize Transmission client: %v\n", err)
		os.Exit(1)
	}
	bot, err := bot.New(conf.TelegramAPIToken, trans,
		bot.WithLogger(log),
		bot.WithAllowedUser(conf.TelegramAllowUser),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize bot: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT)
	go func() {
		<-sig
		log.Infof("got Ctrl-C, stopping")
		cancel()
	}()

	log.Infof("initialized, staring main loop")
	bot.Run(ctx)
}
