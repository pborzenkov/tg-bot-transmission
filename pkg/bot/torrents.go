package bot

import (
	"context"
	"errors"
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

	removeTemplate = template.Must(template.New("remove").Parse(
		`I'm going to remove the following torrents:

{{ range . -}}
\<*{{ .ID }}*\> *{{ .Name }}*
{{ end }}
Should I remove their data files as well?`,
	))
)

func (b *Bot) addTorrent(ctx context.Context, m *tgbotapi.Message,
	req *transmission.AddTorrentReq) (tgbotapi.Chattable, error) {
	if len(b.locations) == 0 {
		torrent, err := b.trans.AddTorrent(ctx, req)
		if err != nil {
			return nil, err
		}

		return reply(m,
			withText(fmt.Sprintf("👌 \\<*%d*\\> %s", torrent.ID, escapeMarkdownV2(torrent.Name))),
			withMarkdownV2(),
			withQuoteMessage(),
		), nil
	}

	id := b.addCallbackHandler(func(ctx context.Context, q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
		var path string
		switch q.Data {
		case "cancel":
			return edit(q.Message, withText("Ok, not gonna download it")), nil
		case "other":
		default:
			var ok bool
			path, ok = b.locations[q.Data]
			if !ok {
				return nil, errors.New("I don't know this location") //nolint:stylecheck
			}

			req.DownloadDirectory = transmission.OptString(path)
		}
		torrent, err := b.trans.AddTorrent(ctx, req)
		if err != nil {
			return nil, err
		}

		if path != "" {
			path = fmt.Sprintf("\n\nWill be downloaded to *%s*", escapeMarkdownV2(path))
		}
		return edit(
			q.Message,
			withText(fmt.Sprintf("👌 \\<*%d*\\> %s%s",
				torrent.ID, escapeMarkdownV2(torrent.Name), path)),
			withMarkdownV2()), nil
	})

	row := make([]tgbotapi.InlineKeyboardButton, 0, len(b.locations)+1)
	for n := range b.locations {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(n, id+n))
	}
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("Other", id+"other"))
	return reply(
		m,
		withText("Ok, gonna queue it for download. But first tell me what is it?"),
		withInlineKeyboard(
			row,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Cancel", id+"cancel"),
			)),
		withQuoteMessage(),
	), nil
}

func (b *Bot) checkPort(ctx context.Context, m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	open, err := b.trans.IsPortOpen(ctx)
	if err != nil {
		return nil, err
	}

	r := "Hooray! The port is open :)"
	if !open {
		r = "Hmm... The port is closed :("
	}

	return reply(m, withText(r)), nil
}

func (b *Bot) stats(ctx context.Context, m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	stats, err := b.trans.GetSessionStats(ctx)
	if err != nil {
		return nil, err
	}
	session, err := b.trans.GetSession(ctx, transmission.SessionFieldTurtleEnabled)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return reply(m, withText(buf.String()), withMarkdownV2()), nil
}

func (b *Bot) setTurtle(ctx context.Context, m *tgbotapi.Message, on bool) (tgbotapi.Chattable, error) {
	if err := b.trans.SetSession(ctx, &transmission.SetSessionReq{
		TurtleEnabled: transmission.OptBool(on),
	}); err != nil {
		return nil, err
	}

	state := "*enabled* 🐢"
	if !on {
		state = "*disabled* 🚀"
	}
	return reply(m, withText("Turtle mode is now "+state), withMarkdownV2()), nil
}

func getTorrentIDs(args string) (transmission.Identifier, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return transmission.All(), nil
	}

	ids := strings.Split(args, " ")
	targets := make([]transmission.SingularIdentifier, 0)
	for _, i := range ids {
		i = strings.TrimSpace(i)
		if i == "" {
			continue
		}
		id, err := strconv.Atoi(i)
		if err != nil {
			return nil, err
		}
		targets = append(targets, transmission.ID(id))
	}
	if len(targets) == 0 {
		return transmission.All(), nil
	}

	return transmission.IDs(targets...), nil
}

func (b *Bot) resumeTorrents(ctx context.Context, m *tgbotapi.Message, args string) (tgbotapi.Chattable, error) {
	ids, err := getTorrentIDs(args)
	if err != nil {
		return nil, err
	}
	if err := b.trans.StartTorrents(ctx, ids); err != nil {
		return nil, err
	}

	return reply(m, withText("Done 😎")), nil
}

func (b *Bot) stopTorrents(ctx context.Context, m *tgbotapi.Message, args string) (tgbotapi.Chattable, error) {
	ids, err := getTorrentIDs(args)
	if err != nil {
		return nil, err
	}
	if err := b.trans.StopTorrents(ctx, ids); err != nil {
		return nil, err
	}

	return reply(m, withText("Done 😎")), nil
}

func (b *Bot) listTorrents(ctx context.Context, m *tgbotapi.Message, args string) (tgbotapi.Chattable, error) {
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
		return nil, err
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
		return nil, err
	}

	return reply(m, withText(buf.String()), withMarkdownV2()), nil
}

func (b *Bot) removeTorrents(ctx context.Context, m *tgbotapi.Message, args string) (tgbotapi.Chattable, error) {
	ids, err := getTorrentIDs(args)
	if err != nil {
		return nil, err
	}

	torrents, err := b.trans.GetTorrents(ctx, ids,
		transmission.TorrentFieldID,
		transmission.TorrentFieldHash,
		transmission.TorrentFieldName,
	)
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return reply(m, withText("Don't have any matching torrents")), nil
	}

	type torrent struct {
		ID   transmission.ID
		Name string
	}
	tors := make([]torrent, 0, len(torrents))
	hashes := make([]transmission.SingularIdentifier, 0, len(torrents))
	for _, t := range torrents {
		tors = append(tors, torrent{ID: t.ID, Name: escapeMarkdownV2(t.Name)})
		hashes = append(hashes, t.Hash)
	}

	buf := new(strings.Builder)
	if err := removeTemplate.Execute(buf, tors); err != nil {
		return nil, err
	}

	id := b.addCallbackHandler(func(ctx context.Context, q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
		var withData bool
		switch q.Data {
		case "yes":
			withData = true
		case "no":
		default:
			return edit(q.Message, withText("Ok, not gonna remove any torrents")), nil
		}
		if err := b.trans.RemoveTorrents(ctx, transmission.IDs(hashes...), withData); err != nil {
			return nil, err
		}

		return edit(q.Message, withText("Done 😎")), nil
	})

	return reply(m, withText(buf.String()), withMarkdownV2(), withInlineKeyboard(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Yes", id+"yes"),
		tgbotapi.NewInlineKeyboardButtonData("No", id+"no"),
		tgbotapi.NewInlineKeyboardButtonData("Cancel", id+"cancel"),
	))), nil
}
