// Package ui embeds the front end.
package ui

import "embed"

//go:embed preview.html kiosk.html manifest.webmanifest assets fonts
var FS embed.FS
