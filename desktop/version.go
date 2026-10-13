package main

// Both values are set when a release is built (see cmd/release):
//
//	wails build -ldflags "-X main.version=1.2.0 -X 'main.updateSource=\\SERVER\Share\SvnBuilder'"
//
// A plain `wails build` leaves them as they are here, which marks a development build that
// never looks for updates.
var (
	// version is the release number shown in Settings.
	version = "dev"
	// updateSource is the folder new releases are published to, and where this build looks for them.
	updateSource = ""
)

func isReleaseBuild() bool {
	return version != "dev" && version != ""
}
