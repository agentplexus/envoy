package openai

import (
	"context"

	"github.com/plexusone/omniagent/api/openai/internal/ogen"
)

// ogenServerHandler bridges ogen Handler interface to AgentHandler.
type ogenServerHandler struct {
	agent AgentHandler
}

var _ ogen.Handler = (*ogenServerHandler)(nil)

// CreateChatCompletion implements ogen.Handler.
func (h *ogenServerHandler) CreateChatCompletion(ctx context.Context, req *ogen.CreateChatCompletionRequest) (ogen.CreateChatCompletionRes, error) {
	// Convert ogen request to our request type
	chatReq := convertOgenRequest(req)

	// Call the agent handler
	resp, err := h.agent.ChatCompletion(ctx, chatReq)
	if err != nil {
		return convertError(err), nil
	}

	// Convert response to ogen type
	return convertToOgenResponse(resp), nil
}

// ListModels implements ogen.Handler.
func (h *ogenServerHandler) ListModels(ctx context.Context) (*ogen.ListModelsResponse, error) {
	models, err := h.agent.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	ogenModels := make([]ogen.Model, len(models))
	for i, m := range models {
		ogenModels[i] = ogen.Model{
			ID:      m.ID,
			Object:  ogen.ModelObjectModel,
			Created: int(m.Created),
			OwnedBy: m.OwnedBy,
		}
	}

	return &ogen.ListModelsResponse{
		Object: ogen.ListModelsResponseObjectList,
		Data:   ogenModels,
	}, nil
}

// RetrieveModel implements ogen.Handler.
func (h *ogenServerHandler) RetrieveModel(ctx context.Context, params ogen.RetrieveModelParams) (ogen.RetrieveModelRes, error) {
	model, err := h.agent.GetModel(ctx, params.Model)
	if err != nil {
		return &ogen.ErrorResponse{
			Error: ogen.ErrorResponseError{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		}, nil
	}

	return &ogen.Model{
		ID:      model.ID,
		Object:  ogen.ModelObjectModel,
		Created: int(model.Created),
		OwnedBy: model.OwnedBy,
	}, nil
}

// convertOgenRequest converts ogen request to our request type.
func convertOgenRequest(req *ogen.CreateChatCompletionRequest) *ChatCompletionRequest {
	chatReq := &ChatCompletionRequest{
		Model:    req.Model,
		Messages: make([]Message, len(req.Messages)),
	}

	// Convert messages
	for i, msg := range req.Messages {
		chatReq.Messages[i] = convertOgenMessage(msg)
	}

	// Convert optional fields
	if req.Temperature.Set && !req.Temperature.Null {
		temp := req.Temperature.Value
		chatReq.Temperature = &temp
	}
	if req.TopP.Set && !req.TopP.Null {
		topP := req.TopP.Value
		chatReq.TopP = &topP
	}
	if req.N.Set && !req.N.Null {
		n := req.N.Value
		chatReq.N = &n
	}
	if req.Stream.Set && !req.Stream.Null {
		chatReq.Stream = req.Stream.Value
	}
	if req.MaxTokens.Set && !req.MaxTokens.Null {
		maxTokens := req.MaxTokens.Value
		chatReq.MaxTokens = &maxTokens
	}
	if req.PresencePenalty.Set && !req.PresencePenalty.Null {
		pp := req.PresencePenalty.Value
		chatReq.PresencePenalty = &pp
	}
	if req.FrequencyPenalty.Set && !req.FrequencyPenalty.Null {
		fp := req.FrequencyPenalty.Value
		chatReq.FrequencyPenalty = &fp
	}
	if req.User.Set {
		chatReq.User = req.User.Value
	}
	if req.Seed.Set && !req.Seed.Null {
		seed := req.Seed.Value
		chatReq.Seed = &seed
	}

	// Convert tools
	if len(req.Tools) > 0 {
		chatReq.Tools = make([]Tool, len(req.Tools))
		for i, t := range req.Tools {
			chatReq.Tools[i] = convertOgenTool(t)
		}
	}

	// Convert stop sequences
	if req.Stop.Set && !req.Stop.Null {
		if req.Stop.Value.IsString() {
			chatReq.Stop = []string{req.Stop.Value.String}
		} else if req.Stop.Value.IsStringArray() {
			chatReq.Stop = req.Stop.Value.StringArray
		}
	}

	return chatReq
}

// convertOgenMessage converts an ogen message to our message type.
func convertOgenMessage(msg ogen.ChatCompletionMessageParam) Message {
	m := Message{
		Role: string(msg.Role),
	}

	// Convert content
	if msg.Content.Set && !msg.Content.Null {
		if msg.Content.Value.IsString() {
			m.Content = msg.Content.Value.String
		} else if msg.Content.Value.IsChatCompletionContentPartArray() {
			// Concatenate text parts
			for _, part := range msg.Content.Value.ChatCompletionContentPartArray {
				if part.Type == ogen.ChatCompletionContentPartTypeText && part.Text.Set {
					m.Content += part.Text.Value
				}
			}
		}
	}

	// Convert name
	if msg.Name.Set {
		m.Name = msg.Name.Value
	}

	// Convert tool_call_id
	if msg.ToolCallID.Set {
		m.ToolCallID = msg.ToolCallID.Value
	}

	// Convert tool_calls
	if len(msg.ToolCalls) > 0 {
		m.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			m.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return m
}

// convertOgenTool converts an ogen tool to our tool type.
func convertOgenTool(t ogen.ChatCompletionTool) Tool {
	tool := Tool{
		Type: string(t.Type),
		Function: Function{
			Name: t.Function.Name,
		},
	}

	if t.Function.Description.Set {
		tool.Function.Description = t.Function.Description.Value
	}

	// Note: FunctionObjectParameters is generated as an empty struct,
	// so we can't easily access the parameters. In practice, the parameters
	// would need custom JSON handling or the OpenAPI spec needs adjustment.

	if t.Function.Strict.Set && !t.Function.Strict.Null {
		tool.Function.Strict = t.Function.Strict.Value
	}

	return tool
}

// convertToOgenResponse converts our response to ogen type.
func convertToOgenResponse(resp *ChatCompletionResponse) *ogen.CreateChatCompletionResponse {
	ogenResp := &ogen.CreateChatCompletionResponse{
		ID:      resp.ID,
		Object:  ogen.CreateChatCompletionResponseObjectChatCompletion,
		Created: int(resp.Created),
		Model:   resp.Model,
		Choices: make([]ogen.ChatCompletionChoice, len(resp.Choices)),
	}

	for i, c := range resp.Choices {
		ogenResp.Choices[i] = convertToOgenChoice(c)
	}

	if resp.Usage != nil {
		ogenResp.Usage = ogen.OptCompletionUsage{
			Set: true,
			Value: ogen.CompletionUsage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			},
		}
	}

	if resp.SystemFingerprint != "" {
		ogenResp.SystemFingerprint = ogen.OptString{
			Set:   true,
			Value: resp.SystemFingerprint,
		}
	}

	return ogenResp
}

// convertToOgenChoice converts our choice to ogen type.
func convertToOgenChoice(c Choice) ogen.ChatCompletionChoice {
	choice := ogen.ChatCompletionChoice{
		Index: c.Index,
		Message: ogen.ChatCompletionMessage{
			Role: ogen.ChatCompletionMessageRoleAssistant,
		},
		FinishReason: ogen.NilChatCompletionChoiceFinishReason{
			Value: ogen.ChatCompletionChoiceFinishReason(c.FinishReason),
			Null:  c.FinishReason == "",
		},
	}

	if c.Message.Content != "" {
		choice.Message.Content = ogen.NilString{
			Value: c.Message.Content,
			Null:  false,
		}
	} else {
		choice.Message.Content = ogen.NilString{Null: true}
	}

	// Convert tool calls
	if len(c.Message.ToolCalls) > 0 {
		choice.Message.ToolCalls = make([]ogen.ChatCompletionMessageToolCall, len(c.Message.ToolCalls))
		for i, tc := range c.Message.ToolCalls {
			choice.Message.ToolCalls[i] = ogen.ChatCompletionMessageToolCall{
				ID:   tc.ID,
				Type: ogen.ChatCompletionMessageToolCallType(tc.Type),
				Function: ogen.ChatCompletionMessageToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return choice
}

// convertError converts an error to ogen error response.
func convertError(err error) ogen.CreateChatCompletionRes {
	errResp := &ogen.CreateChatCompletionInternalServerError{
		Error: ogen.ErrorResponseError{
			Message: err.Error(),
			Type:    "internal_error",
		},
	}
	return errResp
}
