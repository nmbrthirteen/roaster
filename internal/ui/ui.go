// Package ui embeds the front end. go:embed cannot reach outside its own
// package directory, so the markup and print assets live here rather than at
// the module root.
package ui

import "embed"

//go:embed preview.html assets
var FS embed.FS
