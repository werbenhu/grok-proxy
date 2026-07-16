package auth

import "errors"

var (
	ErrAuthorizationPending    = errors.New("等待用户完成 Grok 授权")
	ErrSlowDown                = errors.New("Grok 授权轮询过快")
	ErrAuthorizationDenied     = errors.New("Grok 授权已拒绝或过期")
	ErrCredentialMissing       = errors.New("尚未配置 Grok 凭据")
	ErrReauthorizationRequired = errors.New("Grok 授权已失效，需要重新授权")
)
