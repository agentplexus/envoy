// Package ent holds the generated Ent client for the team system of record.
package ent

// The sql/execquery feature exposes ExecContext on *ent.Tx, which the store
// uses to set per-transaction RLS GUCs (SET LOCAL via set_config).
//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/execquery ./schema
