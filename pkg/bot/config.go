package bot

import (
	"net/http"
)

type config struct {
	Log         Logger
	AllowedUser string
	HTTPClient  *http.Client
}

func defaultConfig() *config {
	return &config{
		Log:        noopLogger{},
		HTTPClient: http.DefaultClient,
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
