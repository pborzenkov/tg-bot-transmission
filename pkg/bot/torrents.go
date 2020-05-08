package bot

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/dustin/go-humanize"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
)

var (
	statsTemplate = template.Must(template.New("stats").Parse(
		`*Rate*: *↓* {{ .DownloadRate }}/s *↑* {{ .UploadRate }}/s \({{ if .TurtleMode }}🐢{{ else }}~🐢~{{ end }}\)
*Torrents*: {{ .TotalTorrents }}, *Active*: {{ .ActiveTorrents }}
*Total*: *↓* {{ .DownloadedTotal }} *↑* {{ .UploadedTotal }} \(☯ {{ .Ratio }}\)`,
	))
)

func (b *Bot) addTorrent(ctx context.Context, m *tgbotapi.Message, req *transmission.AddTorrentReq) tgbotapi.Chattable {
	torrent, err := b.trans.AddTorrent(ctx, req)
	if err != nil {
		return replyError(m, err)
	}

	return replyText(m,
		fmt.Sprintf("Added new torrent\n\n*%d* \\- _%s_", torrent.ID, escapeMarkdownV2(torrent.Name)),
		withParseMode("MarkdownV2"),
	)
}

func (b *Bot) checkPort(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	open, err := b.trans.IsPortOpen(ctx)
	if err != nil {
		return replyError(m, err)
	}

	reply := "Hooray! The port is open :)"
	if !open {
		reply = "Hmm... The port is closed :("
	}

	return replyText(m, reply)
}

func (b *Bot) stats(ctx context.Context, m *tgbotapi.Message) tgbotapi.Chattable {
	stats, err := b.trans.GetSessionStats(ctx)
	if err != nil {
		return replyError(m, err)
	}
	session, err := b.trans.GetSession(ctx)
	if err != nil {
		return replyError(m, err)
	}

	buf := new(strings.Builder)
	if err := statsTemplate.Execute(buf, struct {
		DownloadRate    string
		UploadRate      string
		TurtleMode      bool
		TotalTorrents   int
		ActiveTorrents  int
		DownloadedTotal string
		UploadedTotal   string
		Ratio           string
	}{
		DownloadRate:    escapeMarkdownV2(humanize.IBytes(uint64(stats.DownloadRate))),
		UploadRate:      escapeMarkdownV2(humanize.IBytes(uint64(stats.UploadRate))),
		TurtleMode:      session.TurtleEnabled,
		TotalTorrents:   stats.Torrents,
		ActiveTorrents:  stats.ActiveTorrents,
		DownloadedTotal: escapeMarkdownV2(humanize.IBytes(uint64(stats.AllSessions.Downloaded))),
		UploadedTotal:   escapeMarkdownV2(humanize.IBytes(uint64(stats.AllSessions.Uploaded))),
		Ratio: escapeMarkdownV2(fmt.Sprintf("%.2f",
			float64(stats.AllSessions.Uploaded)/float64(stats.AllSessions.Downloaded))),
	}); err != nil {
		return replyError(m, err)
	}

	return replyText(m, buf.String(), withParseMode("MarkdownV2"))
}

func (b *Bot) setTurtle(ctx context.Context, m *tgbotapi.Message, on bool) tgbotapi.Chattable {
	if err := b.trans.SetSession(ctx, &transmission.SetSessionReq{
		TurtleEnabled: transmission.OptBool(on),
	}); err != nil {
		return replyError(m, err)
	}

	state := "enabled 🐢"
	if !on {
		state = "disabled ~🐢~"
	}
	return replyText(m, "Turtle mode is now "+state, withParseMode("MarkdownV2"))
}
