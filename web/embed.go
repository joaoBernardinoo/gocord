package web

import "embed"

// Files contains the complete browser client so the production binary is self-contained.
//
//go:embed favicon.ico favicon.png bg-removed-logo.png icon-192.png icon-512.png manifest.json
var Files embed.FS

