package service

import (
	"fmt"

	"github.com/werbenhu/grok-proxy/internal/config"
	"github.com/werbenhu/grok-proxy/internal/proxy"
)

const (
	StatusWaiting         = "waiting"
	StatusRunning         = "running"
	StatusStopped         = "stopped"
	StatusError           = "error"
	StatusReauthorization = "reauthorization_required"
)

type State struct {
	Config           config.PublicConfig `json:"config"`
	Running          bool                `json:"running"`
	Status           string              `json:"status"`
	Address          string              `json:"address"`
	OpenAIBaseURL    string              `json:"openaiBaseUrl"`
	AnthropicBaseURL string              `json:"anthropicBaseUrl"`
	LastError        string              `json:"lastError,omitempty"`
	Stats            proxy.Statistics    `json:"stats"`
}

type Settings struct {
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	AuthMode   string `json:"authMode"`
	APIKey     string `json:"apiKey,omitempty"`
	LocalKey   string `json:"localKey,omitempty"`
}

type ConnectionTest struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

func endpointState(cfg config.Config) (string, string, string) {
	address := cfg.Address()
	root := "http://" + address
	return address, root + "/v1", root
}

func formatModelCount(count int) string {
	if count == 0 {
		return "连接成功"
	}
	return fmt.Sprintf("连接成功，发现 %d 个模型", count)
}
