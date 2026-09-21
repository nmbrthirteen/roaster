// Package version names the build. The release workflow overwrites it with
// -ldflags; a build made any other way stays "dev".
package version

// Version is set at build time with:
//
//	-ldflags "-X github.com/upgaming/roaster/internal/version.Version=v1.2.3"
var Version = "dev"
