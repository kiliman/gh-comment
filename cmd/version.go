package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// releaseVersion is the semantic version of this build.
//
// This is the one place to bump on release, and it must match the git tag.
// CI overrides it from the tag at link time (see .github/workflows/release.yml)
// so a tagged build can never disagree with its tag.
var releaseVersion = "0.2.1"

// versionString reports what is actually running: the semantic version plus
// the commit it was built from.
//
// The commit is not hand-maintained. Go records VCS state in the binary for
// any build made inside a git checkout, so a plain `go build` stamps itself —
// which matters here because this extension is built in place rather than
// installed from a release, and "which binary am I running" is otherwise
// unanswerable. A working tree with uncommitted changes is marked dirty,
// since that build corresponds to no commit at all.
func versionString() string {
	revision, commitTime, modified, ok := vcsBuildInfo()
	if !ok {
		return releaseVersion
	}
	return formatVersion(releaseVersion, revision, commitTime, modified)
}

// formatVersion renders the reported version from explicit inputs.
//
// Split from versionString because the VCS stamps are ambient: test binaries
// carry none, so a test calling versionString() can only skip. Keeping the
// formatting pure is what makes the dirty marker and the SHA shortening
// actually testable.
func formatVersion(release, revision, commitTime string, modified bool) string {
	var details []string

	if revision != "" {
		short := revision
		if len(short) > 7 {
			short = short[:7]
		}
		if modified {
			short += "-dirty"
		}
		details = append(details, short)
	}
	if commitTime != "" {
		details = append(details, "committed "+commitTime)
	}

	if len(details) == 0 {
		return release
	}
	return fmt.Sprintf("%s (%s)", release, strings.Join(details, ", "))
}

// vcsBuildInfo pulls the VCS stamps Go embeds at build time.
func vcsBuildInfo() (revision, commitTime string, modified, ok bool) {
	info, available := debug.ReadBuildInfo()
	if !available {
		return "", "", false, false
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			// This is the commit's timestamp, not the time of the build. Go
			// records no build time at all, so calling it one would overstate
			// what the binary actually knows about itself.
			commitTime = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}

	return revision, commitTime, modified, revision != "" || commitTime != ""
}
