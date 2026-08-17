package openai

import (
	"context"
	"errors"
	"testing"

	"github.com/plexusone/omniagent/api/openai/internal/ogen"
)

func TestConvertOgenRequest_Full(t *testing.T) {
	var content ogen.ChatCompletionMessageParamContent
	content.SetString("hello there")

	req := &ogen.CreateChatCompletionRequest{
		Model: "gpt-test",
		Messages: []ogen.ChatCompletionMessageParam{
			{
				Role:       ogen.ChatCompletionMessageParamRoleUser,
				Content:    ogen.OptNilChatCompletionMessageParamContent{Set: true, Value: content},
				Name:       ogen.OptString{Set: true, Value: "alice"},
				ToolCallID: ogen.OptString{Set: true, Value: "call_abc"},
				ToolCalls: []ogen.ChatCompletionMessageToolCall{
					{
						ID:   "call_1",
						Type: ogen.ChatCompletionMessageToolCallTypeFunction,
						Function: ogen.ChatCompletionMessageToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"city":"SF"}`,
						},
					},
				},
			},
		},
		Temperature:      ogen.OptNilFloat64{Set: true, Value: 0.5},
		TopP:             ogen.OptNilFloat64{Set: true, Value: 0.9},
		N:                ogen.OptNilInt{Set: true, Value: 2},
		Stream:           ogen.OptNilBool{Set: true, Value: true},
		MaxTokens:        ogen.OptNilInt{Set: true, Value: 100},
		PresencePenalty:  ogen.OptNilFloat64{Set: true, Value: 0.1},
		FrequencyPenalty: ogen.OptNilFloat64{Set: true, Value: 0.2},
		User:             ogen.OptString{Set: true, Value: "user-1"},
		Seed:             ogen.OptNilInt{Set: true, Value: 42},
		Tools: []ogen.ChatCompletionTool{
			{
				Type: ogen.ChatCompletionToolTypeFunction,
				Function: ogen.FunctionObject{
					Name:        "get_weather",
					Description: ogen.OptString{Set: true, Value: "gets the weather"},
					Strict:      ogen.OptNilBool{Set: true, Value: true},
				},
			},
		},
	}
	req.Stop.Set = true
	req.Stop.Value.SetString("STOP")

	got := convertOgenRequest(req)

	if got.Model != "gpt-test" {
		t.Errorf("Model = %q, want gpt-test", got.Model)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(got.Messages))
	}
	msg := got.Messages[0]
	if msg.Role != "user" || msg.Content != "hello there" || msg.Name != "alice" || msg.ToolCallID != "call_abc" {
		t.Errorf("message = %+v, unexpected", msg)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool calls = %+v, unexpected", msg.ToolCalls)
	}

	if got.Temperature == nil || *got.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", got.Temperature)
	}
	if got.TopP == nil || *got.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", got.TopP)
	}
	if got.N == nil || *got.N != 2 {
		t.Errorf("N = %v, want 2", got.N)
	}
	if !got.Stream {
		t.Error("Stream = false, want true")
	}
	if got.MaxTokens == nil || *got.MaxTokens != 100 {
		t.Errorf("MaxTokens = %v, want 100", got.MaxTokens)
	}
	if got.PresencePenalty == nil || *got.PresencePenalty != 0.1 {
		t.Errorf("PresencePenalty = %v, want 0.1", got.PresencePenalty)
	}
	if got.FrequencyPenalty == nil || *got.FrequencyPenalty != 0.2 {
		t.Errorf("FrequencyPenalty = %v, want 0.2", got.FrequencyPenalty)
	}
	if got.User != "user-1" {
		t.Errorf("User = %q, want user-1", got.User)
	}
	if got.Seed == nil || *got.Seed != 42 {
		t.Errorf("Seed = %v, want 42", got.Seed)
	}
	if len(got.Tools) != 1 || got.Tools[0].Function.Name != "get_weather" || got.Tools[0].Function.Description != "gets the weather" || !got.Tools[0].Function.Strict {
		t.Errorf("Tools = %+v, unexpected", got.Tools)
	}
	if len(got.Stop) != 1 || got.Stop[0] != "STOP" {
		t.Errorf("Stop = %v, want [STOP]", got.Stop)
	}
}

func TestConvertOgenRequest_StopArray(t *testing.T) {
	req := &ogen.CreateChatCompletionRequest{
		Model: "gpt-test",
	}
	req.Stop.Set = true
	req.Stop.Value.SetStringArray([]string{"STOP1", "STOP2"})

	got := convertOgenRequest(req)
	if len(got.Stop) != 2 || got.Stop[0] != "STOP1" || got.Stop[1] != "STOP2" {
		t.Errorf("Stop = %v, want [STOP1 STOP2]", got.Stop)
	}
}

func TestConvertOgenMessage_ContentPartArray(t *testing.T) {
	var content ogen.ChatCompletionMessageParamContent
	content.SetChatCompletionContentPartArray([]ogen.ChatCompletionContentPart{
		{Type: ogen.ChatCompletionContentPartTypeText, Text: ogen.OptString{Set: true, Value: "part one "}},
		{Type: ogen.ChatCompletionContentPartTypeText, Text: ogen.OptString{Set: true, Value: "part two"}},
	})

	msg := ogen.ChatCompletionMessageParam{
		Role:    ogen.ChatCompletionMessageParamRoleUser,
		Content: ogen.OptNilChatCompletionMessageParamContent{Set: true, Value: content},
	}

	got := convertOgenMessage(msg)
	if got.Content != "part one part two" {
		t.Errorf("Content = %q, want 'part one part two'", got.Content)
	}
}

func TestConvertOgenMessage_NoContent(t *testing.T) {
	msg := ogen.ChatCompletionMessageParam{Role: ogen.ChatCompletionMessageParamRoleAssistant}
	got := convertOgenMessage(msg)
	if got.Role != "assistant" || got.Content != "" {
		t.Errorf("message = %+v, want empty content assistant message", got)
	}
}

func TestConvertOgenTool(t *testing.T) {
	tool := ogen.ChatCompletionTool{
		Type: ogen.ChatCompletionToolTypeFunction,
		Function: ogen.FunctionObject{
			Name:        "search",
			Description: ogen.OptString{Set: true, Value: "search the web"},
			Strict:      ogen.OptNilBool{Set: true, Value: false},
		},
	}

	got := convertOgenTool(tool)
	if got.Type != "function" || got.Function.Name != "search" || got.Function.Description != "search the web" {
		t.Errorf("tool = %+v, unexpected", got)
	}
	if got.Function.Strict {
		t.Error("Strict = true, want false")
	}
}

func TestConvertToOgenResponse(t *testing.T) {
	resp := &ChatCompletionResponse{
		ID:                "chatcmpl-1",
		Created:           1234,
		Model:             "gpt-test",
		SystemFingerprint: "fp_123",
		Usage:             &Usage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7},
		Choices: []Choice{
			{
				Index:        0,
				FinishReason: "stop",
				Message:      Message{Role: "assistant", Content: "hi there"},
			},
		},
	}

	got := convertToOgenResponse(resp)
	if got.ID != "chatcmpl-1" || got.Model != "gpt-test" {
		t.Errorf("response = %+v, unexpected", got)
	}
	if !got.Usage.Set || got.Usage.Value.PromptTokens != 3 || got.Usage.Value.TotalTokens != 7 {
		t.Errorf("usage = %+v, unexpected", got.Usage)
	}
	if !got.SystemFingerprint.Set || got.SystemFingerprint.Value != "fp_123" {
		t.Errorf("system_fingerprint = %+v, unexpected", got.SystemFingerprint)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("choices len = %d, want 1", len(got.Choices))
	}
}

func TestConvertToOgenChoice_ToolCallsAndEmptyContent(t *testing.T) {
	choice := Choice{
		Index:        0,
		FinishReason: "tool_calls",
		Message: Message{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "call_1", Type: "function", Function: FunctionCall{Name: "get_weather", Arguments: `{}`}},
			},
		},
	}

	got := convertToOgenChoice(choice)
	if !got.Message.Content.Null {
		t.Error("expected Null content when Message.Content is empty")
	}
	if got.FinishReason.Null {
		t.Error("expected non-null finish reason")
	}
	if got.FinishReason.Value != "tool_calls" {
		t.Errorf("finish reason = %q, want tool_calls", got.FinishReason.Value)
	}
	if len(got.Message.ToolCalls) != 1 || got.Message.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool calls = %+v, unexpected", got.Message.ToolCalls)
	}
}

func TestConvertToOgenChoice_EmptyFinishReason(t *testing.T) {
	choice := Choice{Message: Message{Role: "assistant", Content: "partial"}}
	got := convertToOgenChoice(choice)
	if !got.FinishReason.Null {
		t.Error("expected Null finish reason when FinishReason is empty string")
	}
	if got.Message.Content.Null || got.Message.Content.Value != "partial" {
		t.Errorf("content = %+v, want non-null 'partial'", got.Message.Content)
	}
}

func TestConvertError(t *testing.T) {
	res := convertError(errors.New("boom"))
	errResp, ok := res.(*ogen.CreateChatCompletionInternalServerError)
	if !ok {
		t.Fatalf("expected *ogen.CreateChatCompletionInternalServerError, got %T", res)
	}
	if errResp.Error.Message != "boom" || errResp.Error.Type != "internal_error" {
		t.Errorf("error = %+v, unexpected", errResp.Error)
	}
}

func TestOgenServerHandler_CreateChatCompletion(t *testing.T) {
	mock := newMockAgentHandler()
	h := &ogenServerHandler{agent: mock}

	req := &ogen.CreateChatCompletionRequest{
		Model:    "test-model",
		Messages: []ogen.ChatCompletionMessageParam{{Role: ogen.ChatCompletionMessageParamRoleUser}},
	}

	res, err := h.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion returned error: %v", err)
	}
	resp, ok := res.(*ogen.CreateChatCompletionResponse)
	if !ok {
		t.Fatalf("expected *ogen.CreateChatCompletionResponse, got %T", res)
	}
	if resp.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", resp.Model)
	}
}

func TestOgenServerHandler_CreateChatCompletion_AgentError(t *testing.T) {
	mock := newMockAgentHandler()
	mock.chatCompletionErr = errors.New("agent down")
	h := &ogenServerHandler{agent: mock}

	req := &ogen.CreateChatCompletionRequest{Model: "test-model"}
	res, err := h.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion returned unexpected transport error: %v", err)
	}
	errResp, ok := res.(*ogen.CreateChatCompletionInternalServerError)
	if !ok {
		t.Fatalf("expected *ogen.CreateChatCompletionInternalServerError, got %T", res)
	}
	if errResp.Error.Message != "agent down" {
		t.Errorf("message = %q, want 'agent down'", errResp.Error.Message)
	}
}

func TestOgenServerHandler_RetrieveModel_NotFound(t *testing.T) {
	mock := newMockAgentHandler()
	h := &ogenServerHandler{agent: mock}

	res, err := h.RetrieveModel(context.Background(), ogen.RetrieveModelParams{Model: "missing"})
	if err != nil {
		t.Fatalf("RetrieveModel returned unexpected error: %v", err)
	}
	errResp, ok := res.(*ogen.ErrorResponse)
	if !ok {
		t.Fatalf("expected *ogen.ErrorResponse, got %T", res)
	}
	if errResp.Error.Type != "invalid_request_error" {
		t.Errorf("error type = %q, want invalid_request_error", errResp.Error.Type)
	}
}
