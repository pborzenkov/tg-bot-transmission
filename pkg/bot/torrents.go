package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/pborzenkov/go-transmission/transmission"

	"gopkg.in/tucnak/telebot.v2"
)

func (b *Bot) addTorrent(req *transmission.AddTorrentReq, reply telebot.Recipient) {
	torrent, err := b.trans.AddTorrent(context.Background(), req)
	if err != nil {
		_, _ = b.bot.Send(reply, fmt.Sprintf("Hmm, something is wrong: %v", err))
		return
	}

	_, _ = b.bot.Send(reply, fmt.Sprintf("Added new torrent %q", torrent.Name))
}

func (b *Bot) processMagnet(m *telebot.Message) {
	if m.Text != "" && !strings.HasPrefix(m.Text, "magnet") {
		_, _ = b.bot.Send(m.Sender, "Sorry, don't know what to do with this")
		return
	}

	b.addTorrent(&transmission.AddTorrentReq{
		URL: transmission.OptString(m.Text),
	}, m.Sender)
}

func (b *Bot) processTorrent(m *telebot.Message) {
	body, err := b.bot.GetFile(&m.Document.File)
	if err != nil {
		_, _ = b.bot.Send(m.Sender, fmt.Sprintf("Hmm, something is wrong: %v", err))
		return
	}
	defer body.Close()

	b.addTorrent(&transmission.AddTorrentReq{
		Meta: body,
	}, m.Sender)
}
