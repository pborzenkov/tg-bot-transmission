package bot

import (
	"net/http"

	"github.com/google/uuid"
)

// Location is a named location for torrent contents.
type Location struct {
	Name string
	Path string
}

type config struct {
	Log         Logger
	AllowedUser string
	HTTPClient  *http.Client
	SetCommands bool
	Locations   []Location

	// only for tests
	NewCallbackID func() string
}

func defaultConfig() *config {
	return &config{
		Log:        noopLogger{},
		HTTPClient: http.DefaultClient,
		NewCallbackID: func() string {
			return uuid.New().String()
		},
	}
}

// Option customized bot with optional configuration.
type Option interface {
	apply(*config)
}
type optionFunc func(*config)

func (o optionFunc) apply(c *config) {
	o(c)
}

// WithLogger configures the bot to use l for logging.
func WithLogger(l Logger) Option {
	return optionFunc(func(c *config) {
		if l != nil {
			c.Log = l
		}
	})
}

// WithAllowedUser sets a username of the telegram account that is allowed to
// control the bot.
func WithAllowedUser(user string) Option {
	return optionFunc(func(c *config) {
		c.AllowedUser = user
	})
}

// WithHTTPClient sets an HTTP client for the bot.
func WithHTTPClient(client *http.Client) Option {
	return optionFunc(func(c *config) {
		if client != nil {
			c.HTTPClient = client
		}
	})
}

// WithSetCommands tells the bot to upload up-to-date list of the supported
// commands to the Telegram servers. It's not considered to be a fatal error if
// the upload fails. Alternitevly, this can be done manually via @BotFather.
func WithSetCommands() Option {
	return optionFunc(func(c *config) {
		c.SetCommands = true
	})
}

// WithLocations adds new torrent contents locations.
func WithLocations(l ...Location) Option {
	return optionFunc(func(c *config) {
		c.Locations = append(c.Locations, l...)
	})
}

// withCallbackIDGenerator overwrites default callback ID generator. Private as
// it's intended for tests only.
func withCallbackIDGenerator(gen func() string) Option {
	return optionFunc(func(c *config) {
		if gen != nil {
			c.NewCallbackID = gen
		}
	})
}
