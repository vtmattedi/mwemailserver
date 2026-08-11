package migrations

import "embed"

// FS contains the versioned SQLite migrations used at runtime.
//
//go:embed *.sql
var FS embed.FS
