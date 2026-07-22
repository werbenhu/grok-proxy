package auth

import "errors"

var (
	ErrAuthorizationPending    = errors.New("waiting for user to complete Grok authorization")
	ErrSlowDown                = errors.New("Grok authorization polling too fast")
	ErrAuthorizationDenied     = errors.New("Grok authorization denied or expired")
	ErrCredentialMissing       = errors.New("Grok credential not configured")
	ErrReauthorizationRequired = errors.New("Grok authorization expired; reauthorization required")
)
