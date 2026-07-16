package upstream

import "context"

const (
	ModeAPIKey = "api_key"
	ModeOAuth  = "oauth"
)

type Authorization struct {
	Mode  string
	Token string
}

type CredentialSource interface {
	Authorization(context.Context) (Authorization, error)
}
