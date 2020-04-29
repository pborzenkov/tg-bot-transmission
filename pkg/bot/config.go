package bot

type config struct {
	Log Logger
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
