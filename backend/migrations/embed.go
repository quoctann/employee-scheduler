// Package migrations embeds the SQL migration files into the binary so the
// backend no longer depends on an external migrate CLI/image at deploy time.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
