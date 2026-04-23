package template

import (
	sbClient "github.com/aonesuite/aone/internal/sandbox"
)

// Create builds a Dockerfile as a sandbox template.
//
// Mirrors the e2b CLI `template create <template-name>` command: the template
// name is the only required positional argument, Dockerfile and resource
// options come via flags. It delegates to [Build] so the parse + upload +
// build lifecycle stays in a single place, and defaults to waiting for the
// build to complete.
func Create(info BuildInfo) {
	if info.Name == "" {
		sbClient.PrintError("template name is required")
		return
	}
	if info.Dockerfile == "" {
		sbClient.PrintError("Dockerfile path is required (use -d/--dockerfile)")
		return
	}
	// e2b CLI `template create` defaults to waiting for the build.
	info.Wait = true
	Build(info)
}
