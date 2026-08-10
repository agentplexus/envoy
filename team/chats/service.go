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
	"time"

	"github.com/google/uuid"

	"github.com/plexusone/omniagent/team/ent"
	"github.com/plexusone/omniagent/team/ent/chat"
	"github.com/plexusone/omniagent/team/ent/chatmember"
	"github.com/plexusone/omniagent/team/ent/message"
	entuser "github.com/plexusone/omniagent/team/ent/user"
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
	// ErrNotGroup is returned when a group-only operation targets a chat
	// that is not a group (e.g. inviting into a private DM).
	ErrNotGroup = errors.New("chat is not a group")
	// ErrEmptyName is returned when a group chat is created without a name.
	ErrEmptyName = errors.New("group name is empty")
	// ErrUserNotFound is returned when an invitee username resolves to no user.
	ErrUserNotFound = errors.New("user not found")
	// ErrLastOwner is returned when the sole owner tries to leave a group
	// that still has other members (which would orphan it).
	ErrLastOwner = errors.New("cannot leave: you are the group's only owner")
)

// MaxMessageBytes caps a single message's length (TRD §5).
const MaxMessageBytes = 32 << 10

// MaxChatNameBytes caps a group chat's name.
const MaxChatNameBytes = 200

// Actor identifies the authenticated caller of a group operation. The
// Superadmin flag flows into the store so owner/superadmin administration
// rules resolve correctly under row-level security (a superadmin may
// administer any group's membership, but is not content-privileged).
type Actor struct {
	UserID     uuid.UUID
	Superadmin bool
}

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

// ---- Group chats & membership (RMI-110) ---------------------------------

// CreateGroup creates a group chat owned by the actor, inserting the actor's
// owner membership in the same transaction. Group members added later via
// Invite join as conversants (role "member") with no configuration rights.
func (s *Service) CreateGroup(ctx context.Context, actor Actor, name string) (*ent.Chat, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if len(name) > MaxChatNameBytes {
		return nil, fmt.Errorf("group name exceeds %d bytes", MaxChatNameBytes)
	}
	var c *ent.Chat
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		c, err = tx.Chat.Create().
			SetType(chat.TypeGroup).SetName(name).SetCreatedBy(actor.UserID).Save(ctx)
		if err != nil {
			return err
		}
		_, err = tx.ChatMember.Create().
			SetChatID(c.ID).SetUserID(actor.UserID).SetRole(chatmember.RoleOwner).Save(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}
	return c, nil
}

// ListChats returns the chats the actor is a member of, newest first. RLS (and
// the service's membership scoping) ensures no chat the actor cannot see is
// returned; a superadmin is not content-privileged, so this lists only the
// superadmin's own chats.
func (s *Service) ListChats(ctx context.Context, actor Actor) ([]*ent.Chat, error) {
	var chats []*ent.Chat
	err := s.store.AsUser(ctx, actor.UserID, false, func(ctx context.Context, tx *ent.Tx) error {
		var err error
		// Filter to the actor's memberships explicitly: RLS scopes this on
		// postgres, but the personal-mode SQLite store has no policies.
		chats, err = tx.Chat.Query().
			Where(chat.HasMembersWith(chatmember.UserIDEQ(actor.UserID))).
			Order(ent.Desc(chat.FieldCreatedAt)).All(ctx)
		return err
	})
	return chats, err
}

// GetChat returns a chat the actor is a member of, or ErrForbidden.
func (s *Service) GetChat(ctx context.Context, actor Actor, chatID uuid.UUID) (*ent.Chat, error) {
	var c *ent.Chat
	err := s.store.AsUser(ctx, actor.UserID, false, func(ctx context.Context, tx *ent.Tx) error {
		if err := s.membership(ctx, tx, actor.UserID, chatID); err != nil {
			return err
		}
		var err error
		c, err = tx.Chat.Get(ctx, chatID)
		return err
	})
	if errors.Is(err, ErrForbidden) || ent.IsNotFound(err) {
		return nil, ErrForbidden
	}
	return c, err
}

// Members lists a chat's members (oldest membership first). The actor must be
// a member, or a superadmin (who may administer membership without joining).
func (s *Service) Members(ctx context.Context, actor Actor, chatID uuid.UUID) ([]*ent.ChatMember, error) {
	var members []*ent.ChatMember
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		if err := s.membership(ctx, tx, actor.UserID, chatID); err != nil {
			if !actor.Superadmin {
				return err
			}
		}
		var err error
		members, err = tx.ChatMember.Query().
			Where(chatmember.ChatIDEQ(chatID)).
			Order(ent.Asc(chatmember.FieldJoinedAt)).All(ctx)
		return err
	})
	if errors.Is(err, ErrForbidden) {
		return nil, ErrForbidden
	}
	return members, err
}

// MemberView pairs a membership with the member's username, for display in a
// group's member list (RMI-110/111). Usernames are resolved via system context
// because RLS hides other users' rows from a non-superadmin member; within a
// shared chat, revealing co-members' usernames is expected.
type MemberView struct {
	UserID   uuid.UUID `json:"userId"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

// MembersDetailed lists a chat's members with their usernames. The actor must
// be a member (or superadmin).
func (s *Service) MembersDetailed(ctx context.Context, actor Actor, chatID uuid.UUID) ([]MemberView, error) {
	members, err := s.Members(ctx, actor, chatID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) > 0 {
		if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
			users, err := tx.User.Query().Where(entuser.IDIn(ids...)).All(ctx)
			if err != nil {
				return err
			}
			for _, u := range users {
				names[u.ID] = u.Username
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("resolve member usernames: %w", err)
		}
	}
	out := make([]MemberView, len(members))
	for i, m := range members {
		out[i] = MemberView{UserID: m.UserID, Username: names[m.UserID], Role: m.Role.String(), JoinedAt: m.JoinedAt}
	}
	return out, nil
}

// MemberUserIDs returns the user IDs of a chat's members — the fan-out
// recipient set for a message broadcast. The actor must be a member.
func (s *Service) MemberUserIDs(ctx context.Context, actor Actor, chatID uuid.UUID) ([]uuid.UUID, error) {
	members, err := s.Members(ctx, actor, chatID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids, nil
}

// Invite adds a user (by username) to a group chat as a conversant (member).
// Owner or superadmin only. Idempotent: re-inviting an existing member returns
// the existing membership without change.
func (s *Service) Invite(ctx context.Context, actor Actor, chatID uuid.UUID, username string) (*ent.ChatMember, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return nil, ErrUserNotFound
	}
	if err := s.requireOwner(ctx, actor, chatID); err != nil {
		return nil, err
	}

	// Resolve the invitee via system context: RLS hides other users from a
	// non-superadmin owner, so the owner cannot look them up in their own
	// scope. Only the resolved user ID crosses back — no user data is exposed.
	var inviteeID uuid.UUID
	if err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		u, err := tx.User.Query().Where(entuser.UsernameEQ(username)).Only(ctx)
		if err != nil {
			return err
		}
		inviteeID = u.ID
		return nil
	}); err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("resolve invitee: %w", err)
	}

	var m *ent.ChatMember
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		existing, err := tx.ChatMember.Query().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(inviteeID)).Only(ctx)
		if err == nil {
			m = existing
			return nil
		}
		if !ent.IsNotFound(err) {
			return err
		}
		m, err = tx.ChatMember.Create().
			SetChatID(chatID).SetUserID(inviteeID).SetRole(chatmember.RoleMember).Save(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("invite: %w", err)
	}
	return m, nil
}

// Leave removes the actor's own membership from a group chat (self-leave). The
// sole owner cannot leave while other members remain (it would orphan the
// group); remove the others or the group first.
func (s *Service) Leave(ctx context.Context, actor Actor, chatID uuid.UUID) error {
	err := s.store.AsUser(ctx, actor.UserID, false, func(ctx context.Context, tx *ent.Tx) error {
		self, err := tx.ChatMember.Query().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(actor.UserID)).Only(ctx)
		if err != nil {
			return err // NotFound → not a member → ErrForbidden below
		}
		c, err := tx.Chat.Get(ctx, chatID)
		if err != nil {
			return err
		}
		if c.Type != chat.TypeGroup {
			return ErrNotGroup
		}
		if self.Role == chatmember.RoleOwner {
			otherOwner, err := tx.ChatMember.Query().
				Where(chatmember.ChatIDEQ(chatID), chatmember.RoleEQ(chatmember.RoleOwner), chatmember.UserIDNEQ(actor.UserID)).Exist(ctx)
			if err != nil {
				return err
			}
			if !otherOwner {
				otherMember, err := tx.ChatMember.Query().
					Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDNEQ(actor.UserID)).Exist(ctx)
				if err != nil {
					return err
				}
				if otherMember {
					return ErrLastOwner
				}
			}
		}
		_, err = tx.ChatMember.Delete().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(actor.UserID)).Exec(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}

// RemoveMember removes another member from a group chat. Owner or superadmin
// only. Owners are not removable this way (they leave themselves); use Leave
// to remove yourself.
func (s *Service) RemoveMember(ctx context.Context, actor Actor, chatID, memberID uuid.UUID) error {
	if memberID == actor.UserID {
		return fmt.Errorf("use Leave to remove yourself: %w", ErrForbidden)
	}
	if err := s.requireOwner(ctx, actor, chatID); err != nil {
		return err
	}
	err := s.store.AsUser(ctx, actor.UserID, actor.Superadmin, func(ctx context.Context, tx *ent.Tx) error {
		target, err := tx.ChatMember.Query().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(memberID)).Only(ctx)
		if err != nil {
			return err
		}
		if target.Role == chatmember.RoleOwner {
			return fmt.Errorf("cannot remove an owner: %w", ErrForbidden)
		}
		_, err = tx.ChatMember.Delete().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(memberID)).Exec(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}

// requireOwner verifies the chat is a group and the actor may administer its
// membership: an owner, or a superadmin (who administers via RLS without being
// a member and thus without content access — hence the system-context lookup).
func (s *Service) requireOwner(ctx context.Context, actor Actor, chatID uuid.UUID) error {
	if actor.Superadmin {
		return s.requireGroupSystem(ctx, chatID)
	}
	err := s.store.AsUser(ctx, actor.UserID, false, func(ctx context.Context, tx *ent.Tx) error {
		c, err := tx.Chat.Get(ctx, chatID)
		if err != nil {
			return err // NotFound → not visible → ErrForbidden below
		}
		if c.Type != chat.TypeGroup {
			return ErrNotGroup
		}
		isOwner, err := tx.ChatMember.Query().
			Where(chatmember.ChatIDEQ(chatID), chatmember.UserIDEQ(actor.UserID), chatmember.RoleEQ(chatmember.RoleOwner)).Exist(ctx)
		if err != nil {
			return err
		}
		if !isOwner {
			return ErrForbidden
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}

// requireGroupSystem confirms a chat exists and is a group, using system
// context so a superadmin (not content-privileged) can administer it.
func (s *Service) requireGroupSystem(ctx context.Context, chatID uuid.UUID) error {
	err := s.store.AsSystem(ctx, func(ctx context.Context, tx *ent.Tx) error {
		c, err := tx.Chat.Get(ctx, chatID)
		if err != nil {
			return err
		}
		if c.Type != chat.TypeGroup {
			return ErrNotGroup
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return ErrForbidden
	}
	return err
}
