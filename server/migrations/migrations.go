// Package migrations embeds the SQL migrations, so the binary runs from any folder.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
