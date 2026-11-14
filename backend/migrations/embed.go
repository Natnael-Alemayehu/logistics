// Package migrations embeds the SQL schema migrations so the server can apply
// them at startup.
package migrations

import "embed"

// FS holds the goose migration files.
//
//go:embed *.sql
var FS embed.FS
