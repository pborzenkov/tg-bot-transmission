package bot

type config struct {
	Log Logger

	AllowedUser string
}

func defaultConfig() *config {
	return &config{
		Log: noopLogger{},
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
