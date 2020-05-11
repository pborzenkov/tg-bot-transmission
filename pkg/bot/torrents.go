package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/dustin/go-humanize"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pborzenkov/go-transmission/transmission"
)

var (
	statsTemplate = template.Must(template.New("stats").Parse(
		`↓*{{ .DownloadRate }}/s* ↑*{{ .UploadRate }}/s* {{ if .TurtleMode }}🐢{{ else }}🚀{{ end }}   ` +
			`↻*{{ .ActiveTorrents }}* ⊗*{{ .PausedTorrents }}*   ` +
			`↓*{{ .DownloadedTotal }}* ↑*{{ .UploadedTotal }}* ☯*{{ .Ratio }}*`,
	))

	listTemplate = template.Must(template.New("list").Parse(
		`{{ if .Torrents }}Here is what I got:
{{ range .Torrents }}
\<*{{ .ID }}*\> *{{ .Name }}*
{{ .Status }} *{{ .Valid }}* of *{{ .Wanted }}* \(*{{ .Perc }}%*\)   ` +
			`↓*{{ .DownloadRate }}/s* ↑*{{ .UploadRate }}/s*` +
			`{{ if .Ratio }} ☯*{{ .Ratio }}*{{ end }}` +
			`{{ if .ETA }}   ETA: *{{ .ETA }}*{{ end }}
{{ end }}{{ else }}Don't have any matching torrent{{ end }}`,
	))
)

func (b *Bot) addTorrent(ctx context.Context, m *tgbotapi.Message, req *transmission.AddTorrentReq) tgbotapi.Chattable {
	torrent, err := b.trans.AddTorrent(ctx, req)
	if err != nil {
		return replyError(m, err)
	}

	return replyText(m,
		fmt.Sprintf("👌 \\<*%d*\\> %s", torrent.ID, escapeMarkdownV2(torrent.Name)),
		withMarkdownV2(),
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
		ActiveTorrents  int
		PausedTorrents  int
		DownloadedTotal string
		UploadedTotal   string
		Ratio           string
	}{
		DownloadRate:    escapeMarkdownV2(humanize.IBytes(uint64(stats.DownloadRate))),
		UploadRate:      escapeMarkdownV2(humanize.IBytes(uint64(stats.UploadRate))),
		TurtleMode:      session.TurtleEnabled,
		ActiveTorrents:  stats.ActiveTorrents,
		PausedTorrents:  stats.PausedTorrents,
		DownloadedTotal: escapeMarkdownV2(humanize.IBytes(uint64(stats.AllSessions.Downloaded))),
		UploadedTotal:   escapeMarkdownV2(humanize.IBytes(uint64(stats.AllSessions.Uploaded))),
		Ratio: escapeMarkdownV2(fmt.Sprintf("%.2f",
			float64(stats.AllSessions.Uploaded)/float64(stats.AllSessions.Downloaded))),
	}); err != nil {
		return replyError(m, err)
	}

	return replyText(m, buf.String(), withMarkdownV2())
}

func (b *Bot) setTurtle(ctx context.Context, m *tgbotapi.Message, on bool) tgbotapi.Chattable {
	if err := b.trans.SetSession(ctx, &transmission.SetSessionReq{
		TurtleEnabled: transmission.OptBool(on),
	}); err != nil {
		return replyError(m, err)
	}

	state := "*enabled* 🐢"
	if !on {
		state = "*disabled* 🚀"
	}
	return replyText(m, "Turtle mode is now "+state, withMarkdownV2())
}

func (b *Bot) startStopTorrents(ctx context.Context, m *tgbotapi.Message, args string,
	op func(context.Context, transmission.Identifier) error) tgbotapi.Chattable {
	var target transmission.Identifier

	if args != "" {
		ids := strings.Split(args, " ")
		targets := make([]transmission.SingularIdentifier, 0)
		for _, i := range ids {
			i = strings.TrimSpace(i)
			if i == "" {
				continue
			}
			id, err := strconv.Atoi(i)
			if err != nil {
				return replyError(m, err)
			}
			targets = append(targets, transmission.ID(id))
		}
		if len(targets) > 0 {
			target = transmission.IDs(targets...)
		}
	}
	if err := op(ctx, target); err != nil {
		return replyError(m, err)
	}

	return replyText(m, "Done 😎")
}

func (b *Bot) listTorrents(ctx context.Context, m *tgbotapi.Message, args string) tgbotapi.Chattable {
	torrents, err := b.trans.GetTorrents(ctx, transmission.All(),
		transmission.TorrentFieldID,
		transmission.TorrentFieldName,
		transmission.TorrentFieldStatus,
		transmission.TorrentFieldValidSize,
		transmission.TorrentFieldWantedSize,
		transmission.TorrentFieldDownloadRate,
		transmission.TorrentFieldUploadRate,
		transmission.TorrentFieldUploadRatio,
		transmission.TorrentFieldETA,
	)
	if err != nil {
		return replyError(m, err)
	}

	type torrent struct {
		ID           transmission.ID
		Name         string
		Status       string
		Valid        string
		Wanted       string
		Perc         string
		DownloadRate string
		UploadRate   string
		Ratio        string
		ETA          string
	}
	var res struct {
		Torrents []torrent
	}

	args = strings.ToLower(args)
	for _, t := range torrents {
		if args != "" && !strings.Contains(strings.ToLower(t.Name), args) {
			continue
		}

		status := t.Status.String()
		st, stSize := utf8.DecodeRuneInString(status)
		var eta string
		if t.ETA > 0 {
			eta = t.ETA.String()
		}
		var ratio string
		if t.UploadRatio > 0 {
			ratio = escapeMarkdownV2(fmt.Sprintf("%.2f", t.UploadRatio))
		}
		res.Torrents = append(res.Torrents, torrent{
			ID:           t.ID,
			Name:         escapeMarkdownV2(t.Name),
			Status:       string(unicode.ToTitle(st)) + status[stSize:],
			Valid:        escapeMarkdownV2(humanize.IBytes(uint64(t.ValidSize))),
			Wanted:       escapeMarkdownV2(humanize.IBytes(uint64(t.WantedSize))),
			Perc:         escapeMarkdownV2(fmt.Sprintf("%.1f", float64(t.ValidSize)/float64(t.WantedSize)*100)),
			DownloadRate: escapeMarkdownV2(humanize.IBytes(uint64(t.DownloadRate))),
			UploadRate:   escapeMarkdownV2(humanize.IBytes(uint64(t.UploadRate))),
			Ratio:        ratio,
			ETA:          eta,
		})
	}
	buf := new(strings.Builder)
	if err := listTemplate.Execute(buf, &res); err != nil {
		return replyError(m, err)
	}

	return replyText(m, buf.String(), withMarkdownV2())
}
