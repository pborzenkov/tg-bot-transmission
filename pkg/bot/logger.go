package bot

// Logger defines a logger interface accepted by the bot.
type Logger interface {
	Infof(fmt string, args ...interface{})
	Debugf(fmt string, args ...interface{})
}

type noopLogger struct{}

func (noopLogger) Infof(fmt string, args ...interface{})  {}
func (noopLogger) Debugf(fmt string, args ...interface{}) {}
