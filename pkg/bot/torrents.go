package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
)

func (b *Bot) addTorrent(ctx context.Context, m *tgbotapi.Message, req *transmission.AddTorrentReq) tgbotapi.Chattable {
	torrent, err := b.trans.AddTorrent(ctx, req)
	if err != nil {
		return replyError(m, err)
	}

	return replyText(m, fmt.Sprintf("Added new torrent %q", torrent.Name))
}

func (b *Bot) checkPort(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	open, err := b.trans.IsPortOpen(ctx)
	if err != nil {
		return replyError(m, err)
	}

	reply := "Hooray! The port is open :) 🟢"
	if !open {
		reply = "Hmm... The port is closed :( 🔴"
	}

	return replyText(m, reply)
}
