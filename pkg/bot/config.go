package bot

type config struct {
	Log Logger

	AllowedUsers []string
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

// WithAllowedUser add user to the list of username that are allowed to speak
// with the bot.
func WithAllowedUser(user string) Option {
	return optionFunc(func(c *config) {
		c.AllowedUsers = append(c.AllowedUsers, user)
	})
}
