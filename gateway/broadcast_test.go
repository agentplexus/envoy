package gateway

import (
	"testing"
)

// fakeClient registers a client with a bound user_id and a buffered send
// channel so BroadcastToUsers delivery can be asserted without a real socket.
func fakeClient(g *Gateway, id, userID string) *Client {
	c := &Client{
		ID:       id,
		gateway:  g,
		send:     make(chan *Message, 8),
		done:     make(chan struct{}),
		metadata: map[string]interface{}{},
	}
	if userID != "" {
		c.metadata["user_id"] = userID
	}
	g.registerClient(c)
	return c
}

func received(c *Client) bool {
	select {
	case <-c.send:
		return true
	default:
		return false
	}
}

func TestBroadcastToUsers_MembershipScoped(t *testing.T) {
	g, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	alice := fakeClient(g, "c-alice", "user-alice")
	bob := fakeClient(g, "c-bob", "user-bob")
	carol := fakeClient(g, "c-carol", "user-carol") // not a member
	anon := fakeClient(g, "c-anon", "")             // unauthenticated

	g.BroadcastToUsers([]string{"user-alice", "user-bob"}, NewEventMessage("chat.message", "room", map[string]interface{}{"x": 1}))

	if !received(alice) {
		t.Error("alice (member) did not receive the message")
	}
	if !received(bob) {
		t.Error("bob (member) did not receive the message")
	}
	if received(carol) {
		t.Error("carol (non-member) received the message — cross-chat leak")
	}
	if received(anon) {
		t.Error("unauthenticated client received the message")
	}
}

func TestBroadcastToUsers_MultipleSocketsSameUser(t *testing.T) {
	g, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// One user with two open tabs; both should receive.
	tab1 := fakeClient(g, "c-1", "user-alice")
	tab2 := fakeClient(g, "c-2", "user-alice")

	g.BroadcastToUsers([]string{"user-alice"}, NewEventMessage("chat.message", "room", nil))

	if !received(tab1) || !received(tab2) {
		t.Error("both of a user's sockets should receive the message")
	}
}

func TestBroadcastToUsers_EmptyRecipientsNoop(t *testing.T) {
	g, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := fakeClient(g, "c-1", "user-alice")
	g.BroadcastToUsers(nil, NewEventMessage("chat.message", "room", nil))
	if received(c) {
		t.Error("no message should be delivered for an empty recipient set")
	}
}
