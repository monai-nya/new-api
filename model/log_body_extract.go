package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// messageContentPart is one entry of an OpenAI multimodal content array.
type messageContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// extractContentText pulls plain text out of an OpenAI message "content" field,
// which may be a plain string or an array of typed parts (text/image/etc.).
// Non-text parts (e.g. images) are skipped. json.RawMessage is referenced as a
// type only; parsing goes through common.Unmarshal per project convention.
func extractContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := common.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var parts []messageContentPart
	if err := common.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				sb.WriteString(p.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// extractRequestText parses the client request body and renders the conversation
// as readable plain text, one "[role] text" line per message (covers OpenAI chat
// and Claude message formats). Falls back to the raw body when it is not a
// recognizable chat request.
func extractRequestText(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := common.Unmarshal(body, &req); err != nil {
		return string(body)
	}
	var sb strings.Builder
	for _, m := range req.Messages {
		text := extractContentText(m.Content)
		if text == "" {
			continue
		}
		role := m.Role
		if role == "" {
			role = "user"
		}
		fmt.Fprintf(&sb, "[%s] %s\n", role, text)
	}
	if sb.Len() == 0 {
		return string(body)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// extractResponseText extracts the model's text output from the response body.
// Handles OpenAI streamed SSE ("data: {…}") and non-streamed JSON. Falls back to
// the raw body when the format is unrecognized.
func extractResponseText(data []byte) string {
	if bytes.Contains(data, []byte("data:")) {
		if s := extractSSEContent(data); s != "" {
			return s
		}
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(data, &resp); err == nil && len(resp.Choices) > 0 {
		var sb strings.Builder
		for _, c := range resp.Choices {
			sb.WriteString(extractContentText(c.Message.Content))
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}
	return string(data)
}

// extractSSEContent concatenates the content deltas from an OpenAI SSE stream.
func extractSSEContent(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var sb strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := common.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				sb.WriteString(c.Delta.Content)
			} else if len(c.Message.Content) > 0 {
				sb.WriteString(extractContentText(c.Message.Content))
			}
		}
	}
	return sb.String()
}
