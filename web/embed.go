package web

import "embed"

// Files contains binary and config assets in the assets sub-directory.
//
//go:embed assets/*
var Files embed.FS

