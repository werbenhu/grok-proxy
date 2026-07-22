package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func convertMessagesRequest(body []byte, model string) ([]byte, ResponseOptions, error) {
	var request anthropicRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, ResponseOptions{}, fmt.Errorf("parse Messages request: %w", err)
	}
	if len(request.Messages) == 0 {
		return nil, ResponseOptions{}, errors.New("messages must be a non-empty array")
	}
	if request.MaxTokens <= 0 {
		return nil, ResponseOptions{}, errors.New("max_tokens must be a positive integer")
	}
	for name, value := range map[string]*float64{"temperature": request.Temperature, "top_p": request.TopP} {
		if value != nil && (*value < 0 || *value > 1) {
			return nil, ResponseOptions{}, fmt.Errorf("%s must be between 0 and 1", name)
		}
	}
	for index, sequence := range request.StopSequences {
		if sequence == "" {
			return nil, ResponseOptions{}, fmt.Errorf("stop_sequences[%d] must not be empty", index)
		}
	}
	if !isEmptyJSON(request.TopK) {
		return nil, ResponseOptions{}, errors.New("Messages top_k cannot be mapped to the Responses API")
	}
	thinkingEnabled := false
	if request.Thinking != nil {
		switch request.Thinking.Type {
		case "", "disabled":
		case "enabled", "adaptive":
			thinkingEnabled = true
		default:
			return nil, ResponseOptions{}, fmt.Errorf("unsupported thinking.type=%q", request.Thinking.Type)
		}
	}
	input, inlineInstructions, err := convertAnthropicMessages(request.Messages, anthropicDeclaredToolNames(request.Tools))
	if err != nil {
		return nil, ResponseOptions{}, err
	}
	if len(input) == 0 {
		return nil, ResponseOptions{}, errors.New("messages contain no sendable user or assistant content")
	}
	target := map[string]any{
		"model": model, "input": input, "stream": request.Stream,
		"max_output_tokens": request.MaxTokens, "store": false,
	}
	instructions := make([]string, 0, len(inlineInstructions))
	if system, err := anthropicSystemText(request.System); err != nil {
		return nil, ResponseOptions{}, err
	} else if system != "" {
		instructions = append(instructions, system)
	}
	instructions = append(instructions, inlineInstructions...)
	if len(instructions) > 0 {
		target["instructions"] = strings.Join(instructions, "\n\n")
	}
	copyOptionalNumber(target, "temperature", request.Temperature)
	copyOptionalNumber(target, "top_p", request.TopP)
	if request.Metadata != nil {
		if userID, _ := request.Metadata["user_id"].(string); strings.TrimSpace(userID) != "" {
			target["safety_identifier"] = strings.TrimSpace(userID)
		}
	}
	if request.OutputConfig != nil && request.OutputConfig.Format != nil {
		if request.OutputConfig.Format.Type != "json_schema" || request.OutputConfig.Format.Schema == nil {
			return nil, ResponseOptions{}, errors.New("output_config.format must be a json_schema with a schema")
		}
		target["text"] = map[string]any{"format": map[string]any{"type": "json_schema", "name": "anthropic_output", "schema": request.OutputConfig.Format.Schema}}
	}
	if thinkingEnabled {
		effort := anthropicThinkingEffort(request.Thinking.BudgetTokens)
		if request.OutputConfig != nil && request.OutputConfig.Effort != "" {
			effort = request.OutputConfig.Effort
		}
		switch effort {
		case "minimal":
			effort = "low"
		case "max", "xhigh":
			effort = "high"
		case "low", "medium", "high":
		default:
			return nil, ResponseOptions{}, fmt.Errorf("unsupported output_config.effort=%q", effort)
		}
		target["reasoning"] = map[string]any{"effort": effort, "summary": "detailed"}
		target["include"] = []any{"reasoning.encrypted_content"}
	}
	if len(request.Tools) > 0 {
		tools, err := convertAnthropicTools(request.Tools)
		if err != nil {
			return nil, ResponseOptions{}, err
		}
		target["tools"] = tools
	}
	if len(request.MCPServers) > 0 {
		servers, err := convertAnthropicMCPServers(request.MCPServers)
		if err != nil {
			return nil, ResponseOptions{}, err
		}
		existing, _ := target["tools"].([]any)
		target["tools"] = append(existing, servers...)
	}
	if request.ToolChoice != nil {
		choice, parallel, err := convertAnthropicToolChoice(*request.ToolChoice)
		if err != nil {
			return nil, ResponseOptions{}, err
		}
		target["tool_choice"] = choice
		target["parallel_tool_calls"] = parallel
	}
	converted, err := json.Marshal(target)
	return converted, ResponseOptions{
		AnthropicThinking: thinkingEnabled,
		StopSequences:     append([]string(nil), request.StopSequences...),
	}, err
}

type anthropicRequest struct {
	Model         string             `json:"model"`
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	System        json.RawMessage    `json:"system"`
	Stream        bool               `json:"stream"`
	Temperature   *float64           `json:"temperature"`
	TopP          *float64           `json:"top_p"`
	StopSequences []string           `json:"stop_sequences"`
	Metadata      map[string]any     `json:"metadata"`
	Thinking      *struct {
		Type         string `json:"type"`
		BudgetTokens int    `json:"budget_tokens"`
	} `json:"thinking"`
	TopK         json.RawMessage      `json:"top_k"`
	MCPServers   []anthropicMCPServer `json:"mcp_servers"`
	OutputConfig *struct {
		Effort string `json:"effort"`
		Format *struct {
			Type   string         `json:"type"`
			Schema map[string]any `json:"schema"`
		} `json:"format"`
	} `json:"output_config"`
	Tools      []map[string]json.RawMessage `json:"tools"`
	ToolChoice *anthropicToolChoice         `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicToolChoice struct {
	Type                   string `json:"type"`
	Name                   string `json:"name"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use"`
}

func convertAnthropicMessages(messages []anthropicMessage, declaredTools map[string]struct{}) ([]any, []string, error) {
	input := make([]any, 0, len(messages))
	instructions := make([]string, 0)
	pendingCalls := make(map[string]struct{})
	usedCalls := make(map[string]struct{})
	for messageIndex, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role == "system" || role == "developer" {
			text, err := anthropicSystemText(message.Content)
			if err != nil {
				return nil, nil, fmt.Errorf("messages[%d] invalid %s content: %w", messageIndex, role, err)
			}
			if text != "" {
				instructions = append(instructions, text)
			}
			continue
		}
		if role != "user" && role != "assistant" {
			return nil, nil, fmt.Errorf("Messages API does not support role=%q", message.Role)
		}
		if len(pendingCalls) > 0 && role != "user" {
			return nil, nil, fmt.Errorf("messages[%d] must be a user message containing tool_result", messageIndex)
		}
		var text string
		if json.Unmarshal(message.Content, &text) == nil {
			if len(pendingCalls) > 0 {
				return nil, nil, fmt.Errorf("messages[%d] must return all pending tool_use", messageIndex)
			}
			input = append(input, map[string]any{"type": "message", "role": role, "content": text})
			continue
		}
		var blocks []map[string]json.RawMessage
		if json.Unmarshal(message.Content, &blocks) != nil {
			return nil, nil, fmt.Errorf("messages[%d].content must be a string or content block array", messageIndex)
		}
		hadPending := len(pendingCalls) > 0
		regularBeforeResult := false
		messageParts := make([]any, 0, len(blocks))
		flushMessage := func() {
			if len(messageParts) > 0 {
				input = append(input, map[string]any{"type": "message", "role": role, "content": messageParts})
				messageParts = nil
			}
		}
		for blockIndex, block := range blocks {
			path := fmt.Sprintf("messages[%d].content[%d]", messageIndex, blockIndex)
			var typeName string
			_ = json.Unmarshal(block["type"], &typeName)
			switch typeName {
			case "text":
				regularBeforeResult = regularBeforeResult || len(pendingCalls) > 0
				var value string
				if json.Unmarshal(block["text"], &value) != nil {
					return nil, nil, fmt.Errorf("invalid %s.text", path)
				}
				messageParts = append(messageParts, map[string]any{"type": "input_text", "text": value})
			case "image":
				regularBeforeResult = regularBeforeResult || len(pendingCalls) > 0
				imageURL, err := anthropicImageURL(block["source"])
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", path, err)
				}
				messageParts = append(messageParts, map[string]any{"type": "input_image", "image_url": imageURL})
			case "document":
				regularBeforeResult = regularBeforeResult || len(pendingCalls) > 0
				document, err := anthropicDocument(block)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", path, err)
				}
				messageParts = append(messageParts, document)
			case "tool_use":
				if role != "assistant" {
					return nil, nil, fmt.Errorf("%s tool_use is only allowed in assistant messages", path)
				}
				flushMessage()
				var value struct {
					ID    string         `json:"id"`
					Name  string         `json:"name"`
					Input map[string]any `json:"input"`
				}
				if encoded, _ := json.Marshal(block); json.Unmarshal(encoded, &value) != nil || strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.Name) == "" || value.Input == nil {
					return nil, nil, fmt.Errorf("%s missing valid id, name, or object input", path)
				}
				if _, exists := usedCalls[value.ID]; exists {
					return nil, nil, fmt.Errorf("%s contains duplicate tool_use id %q", path, value.ID)
				}
				arguments, _ := json.Marshal(value.Input)
				input = append(input, map[string]any{"type": "function_call", "call_id": value.ID, "name": value.Name, "arguments": string(arguments)})
				pendingCalls[value.ID] = struct{}{}
				usedCalls[value.ID] = struct{}{}
			case "tool_result":
				if role != "user" {
					return nil, nil, fmt.Errorf("%s tool_result is only allowed in user messages", path)
				}
				flushMessage()
				var toolUseID string
				_ = json.Unmarshal(block["tool_use_id"], &toolUseID)
				if _, exists := pendingCalls[toolUseID]; strings.TrimSpace(toolUseID) == "" || !exists {
					return nil, nil, fmt.Errorf("%s.tool_use_id %q does not match any pending tool_use", path, toolUseID)
				}
				if regularBeforeResult {
					return nil, nil, fmt.Errorf("%s tool_result must precede text, image, or document blocks", path)
				}
				output, err := anthropicToolResult(block["content"], declaredTools)
				if err != nil {
					return nil, nil, fmt.Errorf("%s.content: %w", path, err)
				}
				if raw := block["is_error"]; !isEmptyJSON(raw) {
					var isError bool
					if json.Unmarshal(raw, &isError) != nil {
						return nil, nil, fmt.Errorf("%s.is_error must be a boolean", path)
					}
					if isError {
						output = markAnthropicToolError(output)
					}
				}
				input = append(input, map[string]any{"type": "function_call_output", "call_id": toolUseID, "output": output})
				delete(pendingCalls, toolUseID)
			case "thinking":
				if role != "assistant" {
					return nil, nil, fmt.Errorf("%s thinking is only allowed in assistant messages", path)
				}
				flushMessage()
				var thinking, signature string
				_ = json.Unmarshal(block["thinking"], &thinking)
				_ = json.Unmarshal(block["signature"], &signature)
				item := map[string]any{"type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": thinking}}}
				if signature != "" {
					item["encrypted_content"] = signature
				}
				input = append(input, item)
			case "redacted_thinking":
				if role != "assistant" {
					return nil, nil, fmt.Errorf("%s redacted_thinking is only allowed in assistant messages", path)
				}
				flushMessage()
				var data string
				if json.Unmarshal(block["data"], &data) != nil || data == "" {
					return nil, nil, fmt.Errorf("invalid %s.data", path)
				}
				input = append(input, map[string]any{"type": "reasoning", "encrypted_content": data})
			default:
				return nil, nil, fmt.Errorf("Anthropic content.type=%q is not supported", typeName)
			}
		}
		flushMessage()
		if hadPending && len(pendingCalls) > 0 {
			return nil, nil, fmt.Errorf("messages[%d] must return all pending tool_use", messageIndex)
		}
	}
	if len(pendingCalls) > 0 {
		return nil, nil, errors.New("messages must provide a tool_result for every tool_use")
	}
	return input, instructions, nil
}

func anthropicSystemText(raw json.RawMessage) (string, error) {
	if isEmptyJSON(raw) {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return "", errors.New("system must be a string or text block array")
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "text" {
			return "", fmt.Errorf("system does not support type=%q", block.Type)
		}
		parts = append(parts, block.Text)
	}
	return strings.Join(parts, "\n\n"), nil
}

func anthropicImageURL(raw json.RawMessage) (string, error) {
	var source struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
		URL       string `json:"url"`
	}
	if json.Unmarshal(raw, &source) != nil {
		return "", errors.New("invalid image.source")
	}
	switch source.Type {
	case "base64":
		if source.MediaType == "" || source.Data == "" {
			return "", errors.New("base64 image missing media_type or data")
		}
		return "data:" + source.MediaType + ";base64," + source.Data, nil
	case "url":
		if strings.TrimSpace(source.URL) == "" {
			return "", errors.New("url image missing url")
		}
		return source.URL, nil
	default:
		return "", fmt.Errorf("unsupported image.source.type=%q", source.Type)
	}
}

func anthropicDocument(block map[string]json.RawMessage) (map[string]any, error) {
	var source struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
		URL       string `json:"url"`
	}
	if json.Unmarshal(block["source"], &source) != nil {
		return nil, errors.New("invalid document.source")
	}
	var title string
	_ = json.Unmarshal(block["title"], &title)
	switch source.Type {
	case "text":
		if source.Data == "" {
			return nil, errors.New("text document missing data")
		}
		return map[string]any{"type": "input_text", "text": source.Data}, nil
	case "url":
		if strings.TrimSpace(source.URL) == "" {
			return nil, errors.New("url document missing url")
		}
		value := map[string]any{"type": "input_file", "file_url": source.URL}
		if title != "" {
			value["filename"] = title
		}
		return value, nil
	case "base64":
		if source.MediaType == "" || source.Data == "" {
			return nil, errors.New("base64 document missing media_type or data")
		}
		value := map[string]any{"type": "input_file", "file_data": "data:" + source.MediaType + ";base64," + source.Data}
		if title != "" {
			value["filename"] = title
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported document.source.type=%q", source.Type)
	}
}

func anthropicToolResult(raw json.RawMessage, declaredTools map[string]struct{}) (any, error) {
	if isEmptyJSON(raw) {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var blocks []map[string]json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return "", errors.New("invalid tool_result.content")
	}
	parts := make([]any, 0, len(blocks))
	for _, block := range blocks {
		var typeName string
		_ = json.Unmarshal(block["type"], &typeName)
		switch typeName {
		case "text":
			var value string
			if json.Unmarshal(block["text"], &value) != nil {
				return nil, errors.New("invalid tool_result text")
			}
			parts = append(parts, map[string]any{"type": "input_text", "text": value})
		case "image":
			imageURL, err := anthropicImageURL(block["source"])
			if err != nil {
				return nil, err
			}
			parts = append(parts, map[string]any{"type": "input_image", "image_url": imageURL})
		case "document":
			document, err := anthropicDocument(block)
			if err != nil {
				return nil, err
			}
			parts = append(parts, document)
		case "tool_reference":
			var toolName string
			if json.Unmarshal(block["tool_name"], &toolName) != nil || strings.TrimSpace(toolName) == "" {
				return nil, errors.New("invalid tool_reference.tool_name")
			}
			toolName = strings.TrimSpace(toolName)
			if _, exists := declaredTools[toolName]; !exists {
				return nil, fmt.Errorf("tool_reference references undeclared tool %q", toolName)
			}
			// Responses 没有 Anthropic tool_reference 内容块。Messages 请求中的全部
			// 工具定义已发送给上游，因此用确定性的结果文本保留“搜索命中”语义。
			parts = append(parts, map[string]any{
				"type": "input_text",
				"text": fmt.Sprintf("Tool search matched declared tool %q; its definition is available in this request.", toolName),
			})
		default:
			return nil, fmt.Errorf("tool_result does not support type=%q", typeName)
		}
	}
	return parts, nil
}

func anthropicDeclaredToolNames(tools []map[string]json.RawMessage) map[string]struct{} {
	declared := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		var name string
		_ = json.Unmarshal(tool["name"], &name)
		if name = strings.TrimSpace(name); name != "" {
			declared[name] = struct{}{}
		}
	}
	return declared
}

func markAnthropicToolError(output any) any {
	const prefix = "Tool execution failed: "
	if text, ok := output.(string); ok {
		return prefix + text
	}
	parts, _ := output.([]any)
	return append([]any{map[string]any{"type": "input_text", "text": prefix}}, parts...)
}

func convertAnthropicTools(tools []map[string]json.RawMessage) ([]any, error) {
	result := make([]any, 0, len(tools))
	for index, tool := range tools {
		var typeName string
		_ = json.Unmarshal(tool["type"], &typeName)
		if strings.HasPrefix(typeName, "web_search_") {
			converted, err := convertAnthropicWebSearchTool(tool, index)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
			continue
		}
		if typeName != "" && typeName != "custom" {
			return nil, fmt.Errorf("Anthropic server tool type=%q is not supported", typeName)
		}
		var name, description string
		_ = json.Unmarshal(tool["name"], &name)
		_ = json.Unmarshal(tool["description"], &description)
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("Anthropic tool missing name")
		}
		var schema any = map[string]any{"type": "object", "properties": map[string]any{}}
		if raw := tool["input_schema"]; !isEmptyJSON(raw) {
			if json.Unmarshal(raw, &schema) != nil {
				return nil, fmt.Errorf("invalid input_schema for tool %q", name)
			}
		}
		converted := map[string]any{"type": "function", "name": name, "description": description, "parameters": schema}
		var strict bool
		if raw := tool["strict"]; !isEmptyJSON(raw) {
			if json.Unmarshal(raw, &strict) != nil {
				return nil, fmt.Errorf("strict for tool %q must be a boolean", name)
			}
			converted["strict"] = strict
		}
		result = append(result, converted)
	}
	return result, nil
}

func convertAnthropicWebSearchTool(tool map[string]json.RawMessage, index int) (map[string]any, error) {
	converted := map[string]any{"type": "web_search"}
	for key, raw := range tool {
		switch key {
		case "type", "name", "cache_control":
			continue
		case "max_uses", "allowed_domains", "blocked_domains", "user_location":
			var value any
			if json.Unmarshal(raw, &value) != nil {
				return nil, fmt.Errorf("invalid tools[%d].%s", index, key)
			}
			if key == "allowed_domains" || key == "blocked_domains" {
				domains, ok := value.([]any)
				if !ok {
					return nil, fmt.Errorf("tools[%d].%s must be a string array", index, key)
				}
				if len(domains) > 5 {
					return nil, fmt.Errorf("tools[%d].%s must not exceed 5 domains", index, key)
				}
				for domainIndex, domain := range domains {
					if text, ok := domain.(string); !ok || strings.TrimSpace(text) == "" {
						return nil, fmt.Errorf("tools[%d].%s[%d] must be a non-empty string", index, key, domainIndex)
					}
				}
			}
			converted[key] = value
		default:
			return nil, fmt.Errorf("Grok Build 0.2.99 does not support Anthropic web search field tools[%d].%s", index, key)
		}
	}
	return converted, nil
}

type anthropicMCPServer struct {
	Name               string `json:"name"`
	URL                string `json:"url"`
	AuthorizationToken string `json:"authorization_token"`
}

func convertAnthropicMCPServers(servers []anthropicMCPServer) ([]any, error) {
	result := make([]any, 0, len(servers))
	for index, server := range servers {
		name := strings.TrimSpace(server.Name)
		url := strings.TrimSpace(server.URL)
		if name == "" || url == "" {
			return nil, fmt.Errorf("mcp_servers[%d] missing name or url", index)
		}
		tool := map[string]any{"type": "mcp", "server_label": name, "server_url": url}
		if server.AuthorizationToken != "" {
			tool["authorization"] = server.AuthorizationToken
		}
		result = append(result, tool)
	}
	return result, nil
}

func anthropicThinkingEffort(budget int) string {
	switch {
	case budget > 0 && budget <= 2048:
		return "low"
	case budget > 10000:
		return "high"
	default:
		return "medium"
	}
}

func convertAnthropicToolChoice(choice anthropicToolChoice) (any, bool, error) {
	parallel := !choice.DisableParallelToolUse
	switch choice.Type {
	case "auto", "none":
		return choice.Type, parallel, nil
	case "any":
		return "required", parallel, nil
	case "tool":
		if strings.TrimSpace(choice.Name) == "" {
			return nil, false, errors.New("tool_choice.tool missing name")
		}
		return map[string]any{"type": "function", "name": choice.Name}, parallel, nil
	default:
		return nil, false, fmt.Errorf("unsupported tool_choice.type=%q", choice.Type)
	}
}
