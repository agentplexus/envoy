package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/plexusone/omniagent/config"
	"github.com/plexusone/omniagent/sessions"
	"github.com/plexusone/omnistorage-core/kvs/backend/sqlite"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage conversation sessions",
	Long: `Commands for viewing and managing conversation sessions.

Sessions store conversation history between you and the agent.`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  "Display all stored conversation sessions.",
	RunE:  listSessions,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show session details",
	Long:  "Display details of a specific session including messages.",
	Args:  cobra.ExactArgs(1),
	RunE:  showSession,
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Long:  "Remove a session and its conversation history.",
	Args:  cobra.ExactArgs(1),
	RunE:  deleteSession,
}

var sessionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all sessions",
	Long:  "Remove all sessions. This operation cannot be undone.",
	RunE:  clearSessions,
}

var (
	sessionsDBPath   string
	sessionsShowJSON bool
	sessionsForce    bool
)

func init() {
	// Flags
	sessionsCmd.PersistentFlags().StringVar(&sessionsDBPath, "db", "", "database path (default: storage.path from config, or ~/.local/share/omniagent/data.db)")
	sessionsShowCmd.Flags().BoolVar(&sessionsShowJSON, "json", false, "output as JSON")
	sessionsClearCmd.Flags().BoolVar(&sessionsForce, "force", false, "skip confirmation")

	// Add subcommands
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsClearCmd)

	rootCmd.AddCommand(sessionsCmd)
}

func getSessionStore() (*sessions.Store, func(), error) {
	dbPath := sessionsDBPath
	if dbPath == "" {
		// Prefer whatever gateway run was actually configured to persist
		// to (RMI-OMNIAGENT-007), so `sessions list/show/...` inspects the
		// same file, not just the hardcoded default.
		dbPath = getConfig().Storage.Path
	}
	if dbPath == "" {
		dbPath = config.DefaultStoragePath()
	}

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) { //nolint:gosec // path is from config or default location
		return nil, nil, fmt.Errorf("database not found at %s", dbPath)
	}

	// Open SQLite backend
	backend, err := sqlite.New(sqlite.Config{
		Path: dbPath,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	store := sessions.NewStore(sessions.StoreConfig{
		Backend: backend,
	})

	cleanup := func() {
		backend.Close()
	}

	return store, cleanup, nil
}

func listSessions(cmd *cobra.Command, args []string) error {
	store, cleanup, err := getSessionStore()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx := context.Background()
	ids, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(ids) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tMESSAGES\tCREATED\tUPDATED")

	for _, id := range ids {
		session, err := store.GetIfExists(ctx, id)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			id,
			len(session.Messages),
			session.CreatedAt.Format("2006-01-02 15:04"),
			session.UpdatedAt.Format("2006-01-02 15:04"),
		)
	}

	w.Flush()
	fmt.Printf("\nTotal: %d sessions\n", len(ids))
	return nil
}

func showSession(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	store, cleanup, err := getSessionStore()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx := context.Background()
	session, err := store.GetIfExists(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session %q not found", sessionID)
	}

	if sessionsShowJSON {
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal session: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Session: %s\n", session.ID)
	fmt.Printf("Created: %s\n", session.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", session.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Messages: %d\n", len(session.Messages))

	if session.AgentID != "" {
		fmt.Printf("Agent: %s\n", session.AgentID)
	}

	if len(session.Metadata) > 0 {
		fmt.Printf("Metadata: %v\n", session.Metadata)
	}

	if len(session.Messages) > 0 {
		fmt.Println("\nConversation:")
		fmt.Println("-------------")
		for i, msg := range session.Messages {
			role := string(msg.Role)
			content := msg.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("[%d] %s: %s\n", i+1, role, content)
		}
	}

	return nil
}

func deleteSession(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	store, cleanup, err := getSessionStore()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx := context.Background()

	// Check if session exists
	_, err = store.GetIfExists(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session %q not found", sessionID)
	}

	if err := store.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	fmt.Printf("Deleted session %s\n", sessionID)
	return nil
}

func clearSessions(cmd *cobra.Command, args []string) error {
	store, cleanup, err := getSessionStore()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx := context.Background()
	ids, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(ids) == 0 {
		fmt.Println("No sessions to clear.")
		return nil
	}

	if !sessionsForce {
		fmt.Printf("This will delete %d sessions. Continue? [y/N] ", len(ids))
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	deleted := 0
	for _, id := range ids {
		if err := store.Delete(ctx, id); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete session %s: %v\n", id, err)
		} else {
			deleted++
		}
	}

	fmt.Printf("Deleted %d sessions\n", deleted)
	return nil
}
