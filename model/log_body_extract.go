package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type logContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func joinLogText(parts []string) string {
	filtered := parts[:0]
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "\n")
}

func extractContentText(raw json.RawMessage, response bool) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := common.Unmarshal(raw, &text); err == nil {
		return text
	}

	var parts []logContentPart
	if err := common.Unmarshal(raw, &parts); err != nil {
		var part logContentPart
		if err := common.Unmarshal(raw, &part); err != nil {
			return ""
		}
		parts = []logContentPart{part}
	}

	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		kind := strings.ToLower(part.Type)
		allowed := kind == "" || kind == "text"
		if response {
			allowed = allowed || kind == "output_text"
		} else {
			allowed = allowed || kind == "input_text"
		}
		if allowed && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return joinLogText(texts)
}

func extractMessageUserText(raw json.RawMessage) string {
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := common.Unmarshal(raw, &messages); err != nil {
		return ""
	}
	texts := make([]string, 0, len(messages))
	for _, message := range messages {
		if strings.ToLower(message.Role) != "user" {
			continue
		}
		if text := extractContentText(message.Content, false); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

func extractGeminiUserText(raw json.RawMessage) string {
	var contents []struct {
		Role  string          `json:"role"`
		Parts json.RawMessage `json:"parts"`
	}
	if err := common.Unmarshal(raw, &contents); err != nil {
		return ""
	}
	texts := make([]string, 0, len(contents))
	for _, content := range contents {
		role := strings.ToLower(content.Role)
		if role != "" && role != "user" {
			continue
		}
		if text := extractContentText(content.Parts, false); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

func extractSimpleUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := common.Unmarshal(raw, &text); err == nil {
		return text
	}
	var texts []string
	if err := common.Unmarshal(raw, &texts); err == nil {
		return joinLogText(texts)
	}
	return ""
}

func extractResponsesUserText(raw json.RawMessage) string {
	if text := extractSimpleUserText(raw); text != "" {
		return text
	}

	var items []json.RawMessage
	if err := common.Unmarshal(raw, &items); err != nil {
		items = []json.RawMessage{raw}
	}
	texts := make([]string, 0, len(items))
	for _, itemRaw := range items {
		if text := extractSimpleUserText(itemRaw); text != "" {
			texts = append(texts, text)
			continue
		}
		var item struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Text    string          `json:"text"`
			Content json.RawMessage `json:"content"`
		}
		if err := common.Unmarshal(itemRaw, &item); err != nil {
			continue
		}
		kind := strings.ToLower(item.Type)
		if kind == "input_text" && item.Text != "" {
			texts = append(texts, item.Text)
			continue
		}
		if strings.ToLower(item.Role) != "user" {
			continue
		}
		if text := extractContentText(item.Content, false); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

// extractRequestText returns user-entered text only. System/developer and
// assistant messages, tool calls/results, files, images, and unknown payload
// fields are deliberately excluded. Unknown request formats are not logged.
func extractRequestText(body []byte) string {
	var request struct {
		Messages json.RawMessage   `json:"messages"`
		Contents json.RawMessage   `json:"contents"`
		Input    json.RawMessage   `json:"input"`
		Prompt   json.RawMessage   `json:"prompt"`
		Requests []json.RawMessage `json:"requests"`
	}
	if err := common.Unmarshal(body, &request); err != nil {
		return ""
	}
	if text := extractMessageUserText(request.Messages); text != "" {
		return text
	}
	if text := extractGeminiUserText(request.Contents); text != "" {
		return text
	}
	if text := extractResponsesUserText(request.Input); text != "" {
		return text
	}
	if text := extractSimpleUserText(request.Prompt); text != "" {
		return text
	}
	if len(request.Requests) > 0 {
		texts := make([]string, 0, len(request.Requests))
		for _, batchRequest := range request.Requests {
			if text := extractRequestText(batchRequest); text != "" {
				texts = append(texts, text)
			}
		}
		return joinLogText(texts)
	}
	return ""
}

func extractChoicesText(raw json.RawMessage) string {
	var choices []struct {
		Text    string `json:"text"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		Delta struct {
			Content json.RawMessage `json:"content"`
		} `json:"delta"`
	}
	if err := common.Unmarshal(raw, &choices); err != nil {
		return ""
	}
	texts := make([]string, 0, len(choices))
	for _, choice := range choices {
		if choice.Text != "" {
			texts = append(texts, choice.Text)
		}
		if text := extractContentText(choice.Message.Content, true); text != "" {
			texts = append(texts, text)
		}
		if text := extractContentText(choice.Delta.Content, true); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

func extractResponsesOutputText(raw json.RawMessage) string {
	var outputs []struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := common.Unmarshal(raw, &outputs); err != nil {
		return ""
	}
	texts := make([]string, 0, len(outputs))
	for _, output := range outputs {
		kind := strings.ToLower(output.Type)
		role := strings.ToLower(output.Role)
		if role != "assistant" && !(kind == "message" && role == "") {
			continue
		}
		if text := extractContentText(output.Content, true); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

func extractGeminiResponseText(raw json.RawMessage) string {
	var candidates []struct {
		Content struct {
			Parts json.RawMessage `json:"parts"`
		} `json:"content"`
	}
	if err := common.Unmarshal(raw, &candidates); err != nil {
		return ""
	}
	texts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if text := extractContentText(candidate.Content.Parts, true); text != "" {
			texts = append(texts, text)
		}
	}
	return joinLogText(texts)
}

func extractClaudeResponseText(raw json.RawMessage) string {
	var parts []logContentPart
	if err := common.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.ToLower(part.Type) == "text" && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return joinLogText(texts)
}

func extractJSONResponseText(data []byte) string {
	var response struct {
		Choices    json.RawMessage   `json:"choices"`
		Output     json.RawMessage   `json:"output"`
		Content    json.RawMessage   `json:"content"`
		Candidates json.RawMessage   `json:"candidates"`
		Responses  []json.RawMessage `json:"responses"`
	}
	if err := common.Unmarshal(data, &response); err != nil {
		return ""
	}
	if text := extractChoicesText(response.Choices); text != "" {
		return text
	}
	if text := extractResponsesOutputText(response.Output); text != "" {
		return text
	}
	if text := extractClaudeResponseText(response.Content); text != "" {
		return text
	}
	if text := extractGeminiResponseText(response.Candidates); text != "" {
		return text
	}
	if len(response.Responses) > 0 {
		texts := make([]string, 0, len(response.Responses))
		for _, batchResponse := range response.Responses {
			if text := extractJSONResponseText(batchResponse); text != "" {
				texts = append(texts, text)
			}
		}
		return joinLogText(texts)
	}
	return ""
}

func extractSSEPayloadText(payload []byte) string {
	var event struct {
		Type  string          `json:"type"`
		Delta json.RawMessage `json:"delta"`
		ContentBlock struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content_block"`
		Choices    json.RawMessage `json:"choices"`
		Candidates json.RawMessage `json:"candidates"`
	}
	if err := common.Unmarshal(payload, &event); err != nil {
		return ""
	}
	switch event.Type {
	case "response.output_text.delta":
		var delta string
		if err := common.Unmarshal(event.Delta, &delta); err == nil {
			return delta
		}
	case "content_block_delta":
		var delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := common.Unmarshal(event.Delta, &delta); err == nil && delta.Type == "text_delta" {
			return delta.Text
		}
	case "content_block_start":
		if event.ContentBlock.Type == "text" {
			return event.ContentBlock.Text
		}
	}
	if text := extractChoicesText(event.Choices); text != "" {
		return text
	}
	return extractGeminiResponseText(event.Candidates)
}

func extractSSEContent(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), common.MaxLogBodySizeKB<<10)
	var text strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		text.WriteString(extractSSEPayloadText([]byte(payload)))
	}
	return text.String()
}

// extractResponseText returns model-generated text only. Tool calls, reasoning,
// usage, metadata, and unknown response formats are not logged.
func extractResponseText(data []byte) string {
	if bytes.Contains(data, []byte("data:")) {
		if text := extractSSEContent(data); text != "" {
			return text
		}
	}
	return extractJSONResponseText(data)
}
