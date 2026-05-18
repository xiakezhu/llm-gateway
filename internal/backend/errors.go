package backend

import "errors"

var (
	ErrBackendUnavailable = errors.New("backend unavailable")
	ErrStreamNotSupported = errors.New("stream not supported")
)
