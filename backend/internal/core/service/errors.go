package service

import "errors"

// ErrInvalidInput marks a request-shape problem the caller can fix (bad
// num_days, etc.), as opposed to a downstream/storage failure — the HTTP
// adapter maps it to 400 instead of 500/502.
var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)
