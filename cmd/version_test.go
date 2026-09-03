package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatVersion(t *testing.T) {
	// The point of this string is answering "which binary am I running", which
	// a hardcoded constant cannot do.
	const sha = "5734d63a1b2c3d4e5f60718293a4b5c6d7e8f900"

	tests := []struct {
		name       string
		revision   string
		commitTime string
		modified   bool
		want       string
	}{
		{
			name:       "clean build reports short sha and commit time",
			revision:   sha,
			commitTime: "2026-09-03T18:40:29Z",
			want:       "0.2.0 (5734d63, committed 2026-09-03T18:40:29Z)",
		},
		{
			name:       "a modified tree is marked dirty",
			revision:   sha,
			commitTime: "2026-09-03T18:40:29Z",
			modified:   true,
			// Uncommitted changes mean the binary matches no commit at all,
			// so the SHA alone would be a lie.
			want: "0.2.0 (5734d63-dirty, committed 2026-09-03T18:40:29Z)",
		},
		{
			name:     "revision without a commit time",
			revision: sha,
			want:     "0.2.0 (5734d63)",
		},
		{
			name:       "commit time without a revision",
			commitTime: "2026-09-03T18:40:29Z",
			want:       "0.2.0 (committed 2026-09-03T18:40:29Z)",
		},
		{
			name: "no stamps degrades to the bare version",
			want: "0.2.0",
		},
		{
			name:     "a short revision is not truncated further",
			revision: "abc12",
			want:     "0.2.0 (abc12)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatVersion("0.2.0", tt.revision, tt.commitTime, tt.modified))
		})
	}
}

func TestVersionStringAlwaysCarriesTheSemver(t *testing.T) {
	assert.Contains(t, versionString(), releaseVersion)
}

func TestReleaseVersionIsBareSemver(t *testing.T) {
	// It is interpolated from a git tag with the leading "v" stripped, and the
	// tag is the source of truth. A stray "v" here would render as "v0.2.0"
	// against tag "v0.2.0" in one place and "0.2.0" in another.
	assert.Regexp(t, `^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`, releaseVersion,
		"releaseVersion must be bare semver with no leading v")
}

func TestVcsBuildInfoShape(t *testing.T) {
	revision, commitTime, _, ok := vcsBuildInfo()
	if !ok {
		t.Skip("no build info available")
	}

	if revision != "" {
		assert.Regexp(t, `^[0-9a-f]{40}$`, revision, "vcs.revision should be a full git SHA")
	}
	if commitTime != "" {
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2}T`, commitTime, "vcs.time should be RFC3339")
	}
}

func TestRootCommandExposesTheVersion(t *testing.T) {
	// Guards the wiring: rootCmd.Version must come from versionString(), not a
	// literal that drifts from the tag.
	require.NotEmpty(t, rootCmd.Version)
	assert.Equal(t, versionString(), rootCmd.Version)
	assert.NotEqual(t, "1.0.0", rootCmd.Version,
		"the old hardcoded version must not come back")
}

func TestVersionDoesNotClaimABuildTime(t *testing.T) {
	// Go records the commit's timestamp (vcs.time), not the time of the build.
	// Labelling it "built" was wrong in a way that mattered: two binaries
	// compiled hours apart from the same commit reported identical times, so
	// the field looked like it distinguished builds when it never could.
	got := formatVersion("0.2.0", "5734d63a1b2c3d", "2026-09-03T18:49:33Z", false)

	assert.Contains(t, got, "committed 2026-09-03T18:49:33Z")
	assert.NotContains(t, got, "built", "the binary knows its commit, not when it was compiled")
}
