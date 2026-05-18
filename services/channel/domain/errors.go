package domain

import "errors"

var (
	// ErrAuth is returned when credentials are invalid or revoked.
	ErrAuth = errors.New("channel: authentication failed")

	// ErrRateLimited is returned when the OTA returns a 429 or equivalent.
	ErrRateLimited = errors.New("channel: rate limited")

	// ErrMappingMissing is returned when no mapping exists for the resource.
	ErrMappingMissing = errors.New("channel: mapping missing")

	// ErrValidation is returned when the OTA rejects the payload.
	ErrValidation = errors.New("channel: validation error")

	// ErrTransient is returned for 5xx or network errors.
	ErrTransient = errors.New("channel: transient error")

	// ErrNotImplemented is returned when a capability is not declared.
	ErrNotImplemented = errors.New("channel: not implemented")
)
