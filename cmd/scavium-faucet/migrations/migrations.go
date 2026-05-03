// Package migrations embeds the SQLite schema migrations used by the faucet.
package migrations

import "embed"

// FS contains versioned SQLite migrations for the faucet service.
//
//go:embed *.sql
var FS embed.FS
