package theme

import "embed"

// Files contains the licensed NAIS theme assets bundled into the executable.
//
//go:embed nais/autoindex.css nais/autoindex.js nais/favicon.ico nais/favicon.png
var Files embed.FS
