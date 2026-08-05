package models

// Artifact represents a build output artifact (e.g. dist folder, binaries) produced by a project build.
type Artifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
