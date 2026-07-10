package model

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRequestTextOnlyIncludesUserText(t *testing.T) {
	body := []byte(`{
        "model":"gpt-4.1",
        "messages":[
            {"role":"system","content":"system secret"},
            {"role":"developer","content":"developer secret"},
            {"role":"user","content":[
                {"type":"text","text":"first user message"},
                {"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}
            ]},
            {"role":"assistant","content":"assistant history","tool_calls":[{"function":{"name":"lookup","arguments":"secret"}}]},
            {"role":"tool","content":"tool result"},
            {"role":"user","content":"second user message"}
        ],
        "tools":[{"type":"function","function":{"name":"lookup","description":"tool definition"}}]
    }`)

	text := extractRequestText(body)

	assert.Equal(t, "first user message\nsecond user message", text)
	assert.NotContains(t, text, "system secret")
	assert.NotContains(t, text, "assistant history")
	assert.NotContains(t, text, "tool")
	assert.NotContains(t, text, "base64")
}

func TestExtractRequestTextSupportsResponsesAndGemini(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name: "responses input",
			body: `{
                "input":[
                    {"type":"message","role":"developer","content":[{"type":"input_text","text":"developer"}]},
                    {"type":"message","role":"user","content":[{"type":"input_text","text":"user question"},{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]},
                    {"type":"function_call","name":"lookup","arguments":"secret"}
                ],
                "tools":[{"type":"function","name":"lookup"}]
            }`,
			expected: "user question",
		},
		{
			name: "gemini contents",
			body: `{
                "systemInstruction":{"parts":[{"text":"system"}]},
                "contents":[
                    {"role":"user","parts":[{"text":"gemini question"},{"inlineData":{"mimeType":"image/png","data":"AAAA"}}]},
                    {"role":"model","parts":[{"text":"old answer"},{"functionCall":{"name":"lookup"}}]}
                ],
                "tools":[{"functionDeclarations":[{"name":"lookup"}]}]
            }`,
			expected: "gemini question",
		},
		{
			name:     "completion prompt",
			body:     `{"prompt":["first prompt","second prompt"],"suffix":"ignored"}`,
			expected: "first prompt\nsecond prompt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, extractRequestText([]byte(test.body)))
		})
	}
}

func TestExtractRequestTextDoesNotFallbackToRawPayload(t *testing.T) {
	assert.Empty(t, extractRequestText([]byte(`{"tools":[{"name":"secret-tool"}]}`)))
	assert.Empty(t, extractRequestText([]byte(`not-json`)))
}

func TestExtractResponseTextOnlyIncludesModelText(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name: "chat completion",
			body: `{"choices":[{"message":{"content":"answer","tool_calls":[{"function":{"name":"lookup","arguments":"secret"}}]}}],"usage":{"prompt_tokens":10}}`,
			expected: "answer",
		},
		{
			name: "responses api",
			body: `{"output":[{"type":"reasoning","summary":[{"text":"private reasoning"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"response answer"}]},{"type":"function_call","name":"lookup","arguments":"secret"}]}`,
			expected: "response answer",
		},
		{
			name:     "claude",
			body:     `{"content":[{"type":"thinking","thinking":"private reasoning"},{"type":"text","text":"claude answer"},{"type":"tool_use","name":"lookup","input":{"secret":true}}]}`,
			expected: "claude answer",
		},
		{
			name:     "gemini",
			body:     `{"candidates":[{"content":{"role":"model","parts":[{"text":"gemini answer"},{"functionCall":{"name":"lookup"}}]}}],"usageMetadata":{"promptTokenCount":10}}`,
			expected: "gemini answer",
		},
		{
			name: "mixed sse",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"secret\"}}]}}]}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n" +
				"data: [DONE]\n\n",
			expected: "hello world",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, extractResponseText([]byte(test.body)))
		})
	}

	assert.Empty(t, extractResponseText([]byte(`{"error":{"message":"upstream secret"}}`)))
}

func TestLogBodySizeLimitBoundary(t *testing.T) {
	assert.True(t, isLogBodyWithinLimit(1024, 1))
	assert.False(t, isLogBodyWithinLimit(1025, 1))
	assert.False(t, isLogBodyWithinLimit(1, 0))
	assert.False(t, isLogBodyWithinLimit(1, common.MaxLogBodySizeKB+1))
}

func TestAttachBodiesUsesIndependentLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldRequestEnabled := common.LogRequestBodyEnabled
	oldRequestMaxKB := common.LogRequestBodyMaxKB
	oldResponseEnabled := common.LogResponseBodyEnabled
	oldResponseMaxKB := common.LogResponseBodyMaxKB
	t.Cleanup(func() {
		common.LogRequestBodyEnabled = oldRequestEnabled
		common.LogRequestBodyMaxKB = oldRequestMaxKB
		common.LogResponseBodyEnabled = oldResponseEnabled
		common.LogResponseBodyMaxKB = oldResponseMaxKB
	})
	common.LogRequestBodyEnabled = true
	common.LogRequestBodyMaxKB = 1
	common.LogResponseBodyEnabled = true
	common.LogResponseBodyMaxKB = 1

	t.Run("oversized response does not suppress request", func(t *testing.T) {
		requestBody := []byte(`{"messages":[{"role":"user","content":"keep request"}]}`)
		responseBody := []byte(`{"choices":[{"message":{"content":"` + strings.Repeat("x", 1100) + `"}}]}`)
		context := newLogBodyTestContext(t, requestBody, responseBody)
		other := map[string]interface{}{}

		attachBodiesToOther(context, &other)

		adminInfo, ok := other["admin_info"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "keep request", adminInfo["request_body"])
		assert.NotContains(t, adminInfo, "response_body")
	})

	t.Run("oversized request does not suppress response", func(t *testing.T) {
		requestBody := []byte(`{"messages":[{"role":"user","content":"` + strings.Repeat("x", 1100) + `"}]}`)
		responseBody := []byte(`{"choices":[{"message":{"content":"keep response"}}]}`)
		context := newLogBodyTestContext(t, requestBody, responseBody)
		other := map[string]interface{}{}

		attachBodiesToOther(context, &other)

		adminInfo, ok := other["admin_info"].(map[string]interface{})
		require.True(t, ok)
		assert.NotContains(t, adminInfo, "request_body")
		assert.Equal(t, "keep response", adminInfo["response_body"])
	})
}

func newLogBodyTestContext(t *testing.T, requestBody []byte, responseBody []byte) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
	context.Set(common.KeyRequestBody, requestBody)
	context.Set(common.KeyCapturedResponseBody, bytes.NewBuffer(responseBody))
	t.Cleanup(func() {
		common.CleanupBodyStorage(context)
	})
	return context
}
