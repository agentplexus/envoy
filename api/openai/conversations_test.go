// Copyright 2025 John Wang. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package openai

import (
	"context"
	"testing"

	"github.com/plexusone/omniagent/api/openai/operations"
)

func TestConversationStore_InMemory(t *testing.T) {
	store := NewConversationStore(ConversationStoreConfig{})
	ctx := context.Background()
	userID := "test-user"

	// Test empty list
	convs, err := store.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(convs) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(convs))
	}

	// Test save
	conv := operations.Conversation{
		ID:        "conv_123",
		Title:     "Test Chat",
		Messages:  []operations.ConversationMessage{{Role: "user", Content: "Hello"}},
		CreatedAt: 1000,
		UpdatedAt: 1000,
	}
	if err := store.SaveConversation(ctx, userID, conv); err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Test get
	got, err := store.GetConversation(ctx, userID, "conv_123")
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if got.Title != "Test Chat" {
		t.Errorf("expected title 'Test Chat', got %q", got.Title)
	}
	if len(got.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(got.Messages))
	}

	// Test list
	convs, err = store.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(convs) != 1 {
		t.Errorf("expected 1 conversation, got %d", len(convs))
	}

	// Test delete
	if err := store.DeleteConversation(ctx, userID, "conv_123"); err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}
	_, err = store.GetConversation(ctx, userID, "conv_123")
	if err == nil {
		t.Error("expected error getting deleted conversation")
	}

	// Test list after delete
	convs, err = store.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(convs) != 0 {
		t.Errorf("expected 0 conversations after delete, got %d", len(convs))
	}
}

func TestConversationStore_SyncConversations(t *testing.T) {
	store := NewConversationStore(ConversationStoreConfig{})
	ctx := context.Background()
	userID := "test-user"

	convs := []operations.Conversation{
		{ID: "conv_1", Title: "Chat 1", CreatedAt: 1000, UpdatedAt: 1000},
		{ID: "conv_2", Title: "Chat 2", CreatedAt: 2000, UpdatedAt: 2000},
		{ID: "conv_3", Title: "Chat 3", CreatedAt: 3000, UpdatedAt: 3000},
	}

	if err := store.SyncConversations(ctx, userID, convs); err != nil {
		t.Fatalf("SyncConversations failed: %v", err)
	}

	got, err := store.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(got))
	}
}

func TestConversationStore_UserIsolation(t *testing.T) {
	store := NewConversationStore(ConversationStoreConfig{})
	ctx := context.Background()

	// Save conversation for user1
	if err := store.SaveConversation(ctx, "user1", operations.Conversation{
		ID:    "conv_1",
		Title: "User1 Chat",
	}); err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Save conversation for user2
	if err := store.SaveConversation(ctx, "user2", operations.Conversation{
		ID:    "conv_2",
		Title: "User2 Chat",
	}); err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// User1 should only see their conversation
	convs, _ := store.ListConversations(ctx, "user1")
	if len(convs) != 1 || convs[0].ID != "conv_1" {
		t.Errorf("user1 should only see conv_1, got %v", convs)
	}

	// User2 should only see their conversation
	convs, _ = store.ListConversations(ctx, "user2")
	if len(convs) != 1 || convs[0].ID != "conv_2" {
		t.Errorf("user2 should only see conv_2, got %v", convs)
	}
}
