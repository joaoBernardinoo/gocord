package web

import "embed"

// Files contains the complete browser client so the production binary is self-contained.
//
//go:embed index.html styles.css app.js sw.js favicon.ico manifest.json
var Files embed.FS

