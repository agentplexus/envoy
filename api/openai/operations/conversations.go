// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package operations

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// ConversationMessage represents a message in a conversation.
type ConversationMessage struct {
	Role    string `json:"role" example:"user" doc:"Message role (user, assistant, system)"`
	Content string `json:"content" example:"Hello, how are you?" doc:"Message content"`
}

// Conversation represents a chat conversation.
type Conversation struct {
	ID        string                `json:"id" example:"conv_abc123" doc:"Unique conversation ID"`
	Title     string                `json:"title" example:"Project Discussion" doc:"Conversation title"`
	Messages  []ConversationMessage `json:"messages" doc:"List of messages in the conversation"`
	Model     string                `json:"model,omitempty" example:"gpt-4" doc:"Model used for this conversation"`
	CreatedAt int64                 `json:"createdAt" example:"1706745600000" doc:"Creation timestamp (Unix ms)"`
	UpdatedAt int64                 `json:"updatedAt" example:"1706745600000" doc:"Last update timestamp (Unix ms)"`
}

// ConversationHandler defines the interface for conversation operations.
type ConversationHandler interface {
	// ListConversations returns all conversations for the user.
	ListConversations(ctx context.Context, userID string) ([]Conversation, error)
	// GetConversation returns a single conversation by ID.
	GetConversation(ctx context.Context, userID, conversationID string) (*Conversation, error)
	// SaveConversation creates or updates a conversation.
	SaveConversation(ctx context.Context, userID string, conv Conversation) error
	// DeleteConversation removes a conversation.
	DeleteConversation(ctx context.Context, userID, conversationID string) error
	// SyncConversations syncs multiple conversations at once.
	SyncConversations(ctx context.Context, userID string, convs []Conversation) error
}

// ListConversationsInput is the input for listing conversations.
type ListConversationsInput struct {
	UserID string `header:"X-User-ID" doc:"User identifier for conversation ownership"`
}

// ListConversationsOutput is the response for listing conversations.
type ListConversationsOutput struct {
	Body struct {
		Conversations []Conversation `json:"conversations" doc:"List of conversations"`
	}
}

// GetConversationInput is the input for getting a conversation.
type GetConversationInput struct {
	UserID         string `header:"X-User-ID" doc:"User identifier for conversation ownership"`
	ConversationID string `path:"id" doc:"Conversation ID"`
}

// GetConversationOutput is the response for getting a conversation.
type GetConversationOutput struct {
	Body Conversation
}

// SaveConversationInput is the input for saving a conversation.
type SaveConversationInput struct {
	UserID string `header:"X-User-ID" doc:"User identifier for conversation ownership"`
	Body   Conversation
}

// SaveConversationOutput is the response for saving a conversation.
type SaveConversationOutput struct {
	Body struct {
		ID        string `json:"id" doc:"Conversation ID"`
		UpdatedAt int64  `json:"updatedAt" doc:"Update timestamp (Unix ms)"`
	}
}

// DeleteConversationInput is the input for deleting a conversation.
type DeleteConversationInput struct {
	UserID         string `header:"X-User-ID" doc:"User identifier for conversation ownership"`
	ConversationID string `path:"id" doc:"Conversation ID"`
}

// DeleteConversationOutput is the response for deleting a conversation.
type DeleteConversationOutput struct {
	Body struct {
		Deleted bool `json:"deleted" doc:"Whether the conversation was deleted"`
	}
}

// SyncConversationsInput is the input for syncing conversations.
type SyncConversationsInput struct {
	UserID string `header:"X-User-ID" doc:"User identifier for conversation ownership"`
	Body   struct {
		Conversations []Conversation `json:"conversations" doc:"Conversations to sync"`
	}
}

// SyncConversationsOutput is the response for syncing conversations.
type SyncConversationsOutput struct {
	Body struct {
		Synced    int   `json:"synced" doc:"Number of conversations synced"`
		UpdatedAt int64 `json:"updatedAt" doc:"Sync timestamp (Unix ms)"`
	}
}

// RegisterConversationOperations registers conversation endpoints.
func RegisterConversationOperations(api huma.API, handler ConversationHandler, prefix string) {
	// List conversations
	huma.Register(api, huma.Operation{
		OperationID:   "listConversations",
		Method:        http.MethodGet,
		Path:          prefix + "/conversations",
		Summary:       "List conversations",
		Description:   "Returns all conversations for the authenticated user.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *ListConversationsInput) (*ListConversationsOutput, error) {
		userID := input.UserID
		if userID == "" {
			userID = "default"
		}

		convs, err := handler.ListConversations(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list conversations", err)
		}

		out := &ListConversationsOutput{}
		out.Body.Conversations = convs
		if out.Body.Conversations == nil {
			out.Body.Conversations = []Conversation{}
		}
		return out, nil
	})

	// Get conversation
	huma.Register(api, huma.Operation{
		OperationID:   "getConversation",
		Method:        http.MethodGet,
		Path:          prefix + "/conversations/{id}",
		Summary:       "Get conversation",
		Description:   "Returns a single conversation by ID.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *GetConversationInput) (*GetConversationOutput, error) {
		userID := input.UserID
		if userID == "" {
			userID = "default"
		}

		conv, err := handler.GetConversation(ctx, userID, input.ConversationID)
		if err != nil {
			return nil, huma.Error404NotFound("conversation not found")
		}

		return &GetConversationOutput{Body: *conv}, nil
	})

	// Save conversation
	huma.Register(api, huma.Operation{
		OperationID:   "saveConversation",
		Method:        http.MethodPut,
		Path:          prefix + "/conversations/{id}",
		Summary:       "Save conversation",
		Description:   "Creates or updates a conversation.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *SaveConversationInput) (*SaveConversationOutput, error) {
		userID := input.UserID
		if userID == "" {
			userID = "default"
		}

		conv := input.Body
		conv.UpdatedAt = time.Now().UnixMilli()

		if err := handler.SaveConversation(ctx, userID, conv); err != nil {
			return nil, huma.Error500InternalServerError("failed to save conversation", err)
		}

		out := &SaveConversationOutput{}
		out.Body.ID = conv.ID
		out.Body.UpdatedAt = conv.UpdatedAt
		return out, nil
	})

	// Delete conversation
	huma.Register(api, huma.Operation{
		OperationID:   "deleteConversation",
		Method:        http.MethodDelete,
		Path:          prefix + "/conversations/{id}",
		Summary:       "Delete conversation",
		Description:   "Deletes a conversation by ID.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *DeleteConversationInput) (*DeleteConversationOutput, error) {
		userID := input.UserID
		if userID == "" {
			userID = "default"
		}

		if err := handler.DeleteConversation(ctx, userID, input.ConversationID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete conversation", err)
		}

		out := &DeleteConversationOutput{}
		out.Body.Deleted = true
		return out, nil
	})

	// Sync conversations (bulk upsert)
	huma.Register(api, huma.Operation{
		OperationID:   "syncConversations",
		Method:        http.MethodPost,
		Path:          prefix + "/conversations/sync",
		Summary:       "Sync conversations",
		Description:   "Syncs multiple conversations at once. Used for initial load and bulk updates.",
		Tags:          []string{"Conversations"},
		DefaultStatus: http.StatusOK,
	}, func(ctx context.Context, input *SyncConversationsInput) (*SyncConversationsOutput, error) {
		userID := input.UserID
		if userID == "" {
			userID = "default"
		}

		if err := handler.SyncConversations(ctx, userID, input.Body.Conversations); err != nil {
			return nil, huma.Error500InternalServerError("failed to sync conversations", err)
		}

		out := &SyncConversationsOutput{}
		out.Body.Synced = len(input.Body.Conversations)
		out.Body.UpdatedAt = time.Now().UnixMilli()
		return out, nil
	})
}
