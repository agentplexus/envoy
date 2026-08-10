// Package chats is the chat/membership/message service layer: DM (private)
// chats with the agent, message persistence, and the agent turn. Group
// chats, the @-mention policy, and the INIT-OMNIAGENT-005 agent registry
// (Can(CapCreateChat), chats.agent_id) are out of scope here — this is the
// personal-mode slice (TRD §5): one implicit user, one private chat, the
// agent always responds.
package chats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/chat"
	"github.com/plexusone/omniagent/team/ent/chatmember"
	"github.com/plexusone/omniagent/team/ent/message"
	"github.com/plexusone/omniagent/team/store"
)

// Sentinel errors returned by the service layer.
var (
	// ErrForbidden is returned when the actor is not a member of the chat.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is returned when the referenced chat does not exist (or
	// is invisible to the actor — indistinguishable by design).
	ErrNotFound = errors.New("not found")
	// ErrEmptyMessage is returned for blank message content.
	ErrEmptyMessage = errors.New("message content is empty")
)

// MaxMessageBytes caps a single message's length (TRD §5).
const MaxMessageBytes = 32 << 10

// AgentProcessor runs one conversational turn. *agent.Agent satisfies this
// structurally; chats stays decoupled from the agent package.
type AgentProcessor interface {
	Process(ctx context.Context, sessionID, content string) (string, error)
}

// Config configures the chats service.
type Config struct {
	// Agent runs the agent's turn on each user message. Nil echoes the
	// message back instead (matches the gateway's no-API-key fallback).
	Agent AgentProcessor
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Service exposes chat operations over the store.
type Service struct {
	store  *store.Store
	agent  AgentProcessor
	logger *slog.Logger
}

// NewService creates the chats service.
func NewService(st *store.Store, cfg Config) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("chats: store is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Service{store: st, agent: cfg.Agent, logger: cfg.Logger}, nil
}

// SessionID returns the agent session key for a chat (TRD §5): one session
// per chat, shared by every member.
func SessionID(chatID uuid.UUID) string {
	return "chat:" + chatID.String()
}

// PrivateChat returns the user's private (DM) chat with the agent, creating
// it on first use. At most one private chat exists per user, enforced by
// the store's partial unique index on (created_by) where type='private'.
func (s *Service) PrivateChat(ctx context.Context, userID uuid.UUID) (*ent.Chat, error) {
	var c *ent.Chat
	err := s.store.AsUser(ctx, userID, false, func(ctx context.Context, tx *ent.Tx) error {
		existing, err := tx.Chat.Query().
			Where(chat.TypeEQ(chat.TypePrivate), chat.CreatedByEQ(userID)).
			Only(ctx)
		if err == nil {
			c = existing
			return nil
		}
		if !ent.IsNotFound(err) {
			return err
		}

		c, err = tx.Chat.Create().SetType(chat.TypePrivate).SetCreatedBy(userID).Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.ChatMember.Create().
			SetChatID(c.ID).SetUserID(userID).SetRole(chatmember.RoleOwner).Save(ctx)
		return err
	})
	return c, err
}

// membership confirms userID is a member of chatID, returning ErrForbidden
// (not found) otherwise — membership state, not chat existence, is what a
// caller may probe.
func (s *Service) membership(ctx context.Context, tx *ent.Tx, userID, chatID uuid.UUID) error {
	exists, err := tx.ChatMember.Query().
		Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(userID)).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return ErrForbidden
	}
	return nil
}

// History returns up to limit messages for chatID, oldest-first. cursor, if
// non-nil, excludes messages at or before that message ID's position
// (keyset pagination by created_at; TRD §5 "Ordering/limits").
func (s *Service) History(ctx context.Context, userID, chatID uuid.UUID, cursor *uuid.UUID, limit int) ([]*ent.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var msgs []*ent.Message
	err := s.store.AsUser(ctx, userID, false, func(ctx context.Context, tx *ent.Tx) error {
		if err := s.membership(ctx, tx, userID, chatID); err != nil {
			return err
		}
		q := tx.Message.Query().Where(message.ChatIDEQ(chatID))
		if cursor != nil {
			exists, err := tx.Message.Query().
				Where(message.IDEQ(*cursor), message.ChatIDEQ(chatID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				return ErrForbidden
			}
			// IDs are UUIDv7 (time-ordered), so the ID alone is a stable
			// keyset cursor — no need to also compare created_at, whose
			// storage precision varies by dialect.
			q = q.Where(message.IDGT(*cursor))
		}
		var err error
		msgs, err = q.Order(ent.Asc(message.FieldID)).Limit(limit).All(ctx)
		return err
	})
	if errors.Is(err, ErrForbidden) || ent.IsNotFound(err) {
		return nil, ErrForbidden
	}
	return msgs, err
}

// HistoryBefore returns up to limit messages older than before, oldest-first
// (ready to prepend in a scroll-back UI — TRD §5 "Ordering/limits"). A nil
// before returns the newest limit messages. This is the backward
// (scroll-back) direction; History is the forward direction.
func (s *Service) HistoryBefore(ctx context.Context, userID, chatID uuid.UUID, before *uuid.UUID, limit int) ([]*ent.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var msgs []*ent.Message
	err := s.store.AsUser(ctx, userID, false, func(ctx context.Context, tx *ent.Tx) error {
		if err := s.membership(ctx, tx, userID, chatID); err != nil {
			return err
		}
		q := tx.Message.Query().Where(message.ChatIDEQ(chatID))
		if before != nil {
			exists, err := tx.Message.Query().
				Where(message.IDEQ(*before), message.ChatIDEQ(chatID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !exists {
				return ErrForbidden
			}
			// IDs are UUIDv7 (time-ordered), so the ID alone is a stable
			// keyset cursor (see History for the rationale).
			q = q.Where(message.IDLT(*before))
		}
		// Fetch the newest `limit` older-than-cursor (descending), then
		// reverse to oldest-first for the caller.
		var err error
		msgs, err = q.Order(ent.Desc(message.FieldID)).Limit(limit).All(ctx)
		if err != nil {
			return err
		}
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
		return nil
	})
	if errors.Is(err, ErrForbidden) || ent.IsNotFound(err) {
		return nil, ErrForbidden
	}
	return msgs, err
}

// Send persists the user's message, runs the agent's turn (private chats
// always respond — TRD §5), persists the reply, and returns both. agentMsg
// is nil only if the agent turn itself errors after the user message
// already landed; callers should still show the user's message. Send is the
// synchronous convenience path; the HTTP layer instead calls PostUserMessage
// then GenerateReply so the (slow) agent turn can be delivered out-of-band.
func (s *Service) Send(ctx context.Context, userID, chatID uuid.UUID, content string) (userMsg, agentMsg *ent.Message, err error) {
	userMsg, err = s.PostUserMessage(ctx, userID, chatID, content)
	if err != nil {
		return nil, nil, err
	}
	agentMsg, err = s.GenerateReply(ctx, chatID, userMsg.Content)
	if err != nil {
		return userMsg, nil, err
	}
	return userMsg, agentMsg, nil
}

// PostUserMessage validates and persists a user message, returning it. It
// performs the membership check but does not run the agent turn — callers
// wanting the reply call GenerateReply next (see Send).
func (s *Service) PostUserMessage(ctx context.Context, userID, chatID uuid.UUID, content string) (*ent.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrEmptyMessage
	}
	if len(content) > MaxMessageBytes {
		return nil, fmt.Errorf("message exceeds %d bytes", MaxMessageBytes)
	}

	var userMsg *ent.Message
	err := s.store.AsUser(ctx, userID, false, func(ctx context.Context, tx *ent.Tx) error {
		if err := s.membership(ctx, tx, userID, chatID); err != nil {
			return err
		}
		var err error
		userMsg, err = tx.Message.Create().
			SetChatID(chatID).SetAuthorType(message.AuthorTypeUser).
			SetAuthorUserID(userID).SetContent(content).Save(ctx)
		return err
	})
	if errors.Is(err, ErrForbidden) || ent.IsNotFound(err) {
		return nil, ErrForbidden
	}
	if err != nil {
		return nil, fmt.Errorf("persist message: %w", err)
	}
	return userMsg, nil
}

// GenerateReply runs the agent's turn for content and persists the reply as
// an agent-authored message (no membership check — the caller already
// validated it via PostUserMessage). The reply row is written AsSystem
// because the agent is not a user principal.
func (s *Service) GenerateReply(ctx context.Context, chatID uuid.UUID, content string) (*ent.Message, error) {
	reply, aerr := s.reply(ctx, chatID, content)
	if aerr != nil {
		return nil, fmt.Errorf("agent turn: %w", aerr)
	}
	var agentMsg *ent.Message
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		agentMsg, err = tx.Message.Create().
			SetChatID(chatID).SetAuthorType(message.AuthorTypeAgent).SetContent(reply).Save(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("persist agent reply: %w", err)
	}
	return agentMsg, nil
}

// reply runs the agent's turn, echoing the message when no agent is
// configured (matches the gateway's no-API-key fallback).
func (s *Service) reply(ctx context.Context, chatID uuid.UUID, content string) (string, error) {
	if s.agent == nil {
		return "Message received: " + content, nil
	}
	return s.agent.Process(ctx, SessionID(chatID), content)
}
