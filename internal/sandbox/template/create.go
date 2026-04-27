package template

import (
	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// Create builds a Dockerfile as a sandbox template.
//
// The template name is the only required positional argument; Dockerfile and
// resource options come via flags. It delegates to [Build] so the parse +
// upload + build lifecycle stays in a single place, and defaults to waiting
// for the build to complete.
func Create(info BuildInfo) {
	if info.Name == "" {
		sbClient.PrintError("template name is required")
		return
	}
	// `template create` defaults to waiting for the build to complete so
	// users get a single command that produces a ready-to-use template.
	info.Wait = true
	Build(info)
}
