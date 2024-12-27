package driver

// ConfigError is a custom error type for configuration errors
type ConfigError struct {
	msg string
}

func NewConfigError(msg string) *ConfigError {
	return &ConfigError{msg}
}

func (e *ConfigError) Error() string {
	return e.msg
}
