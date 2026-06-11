package channels

import "log/slog"

type loggerAwareChannel interface {
	SetLogger(*slog.Logger)
}

func setChannelLogger(channel DeliveryChannel, logger *slog.Logger) {
	if channel == nil {
		return
	}
	aware, ok := channel.(loggerAwareChannel)
	if !ok {
		return
	}
	aware.SetLogger(logger)
}
