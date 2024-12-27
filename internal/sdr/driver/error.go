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

// RuntimeError is a custom error type for runtime errors
type RuntimeError struct {
	msg string
}

func NewRuntimeError(msg string) *RuntimeError {
	return &RuntimeError{msg}
}

func (e *RuntimeError) Error() string {
	return e.msg
}
