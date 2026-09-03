package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/silouanwright/gh-comment/internal/github"
)

// withGlobals saves and restores the flag globals a PR-context test perturbs.
func withGlobals(t *testing.T, repository string, pr int) {
	t.Helper()
	originalRepo, originalPR := repo, prNumber
	repo, prNumber = repository, pr
	t.Cleanup(func() { repo, prNumber = originalRepo, originalPR })
}

func TestGetPRContextForCommentPrefersExplicitPR(t *testing.T) {
	// --pr always wins: it is the escape hatch if inference ever misbehaves.
	withGlobals(t, "owner/repo", 999)
	client := github.NewMockClient()
	client.CommentPRNumber = 42
	calls := stubGhExec(t, "77\n", nil)

	gotRepo, pr, err := getPRContextForComment(client, 555)

	require.NoError(t, err)
	assert.Equal(t, "owner/repo", gotRepo)
	assert.Equal(t, 999, pr, "--pr should beat both the comment and the branch")
	assert.Empty(t, *calls, "an explicit --pr should not shell out at all")
}

func TestGetPRContextForCommentPrefersCommentOverBranch(t *testing.T) {
	// The comment ID is authoritative about which PR it lives on; the branch is
	// a guess that is merely often right. Preferring the guess is what lands
	// writes on the wrong PR.
	withGlobals(t, "owner/repo", 0)
	client := github.NewMockClient()
	client.CommentPRNumber = 42
	calls := stubGhExec(t, "77\n", nil)

	gotRepo, pr, err := getPRContextForComment(client, 555)

	require.NoError(t, err)
	assert.Equal(t, "owner/repo", gotRepo)
	assert.Equal(t, 42, pr, "should use the PR the comment reports, not the branch's")
	assert.Empty(t, *calls, "a successful comment lookup makes branch detection unnecessary")
}

func TestGetPRContextForCommentFallsBackToBranch(t *testing.T) {
	// Nothing that works today should stop working.
	withGlobals(t, "owner/repo", 0)
	client := github.NewMockClient()
	client.GetPRNumberForCommentError = fmt.Errorf("comment #555 not found")
	calls := stubGhExec(t, "77\n", nil)

	_, pr, err := getPRContextForComment(client, 555)

	require.NoError(t, err)
	assert.Equal(t, 77, pr)
	require.Len(t, *calls, 1)
	assert.Contains(t, (*calls)[0], "--repo", "the fallback must still scope itself to the resolved repo")
}

func TestGetPRContextForCommentReportsBothFailures(t *testing.T) {
	withGlobals(t, "owner/repo", 0)
	client := github.NewMockClient()
	client.GetPRNumberForCommentError = fmt.Errorf("comment #555 not found")
	stubGhExec(t, "", fmt.Errorf("exit status 1"))

	_, _, err := getPRContextForComment(client, 555)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "comment lookup", "should say the comment lookup was tried")
	assert.Contains(t, err.Error(), "current branch", "should say branch detection was tried")
	assert.Contains(t, err.Error(), "--pr", "should point at the escape hatch")
}

func TestSplitRepo(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		wantOwner  string
		wantName   string
		wantErr    bool
	}{
		{name: "valid", repository: "owner/repo", wantOwner: "owner", wantName: "repo"},
		{name: "no slash", repository: "ownerrepo", wantErr: true},
		{name: "too many parts", repository: "a/b/c", wantErr: true},
		{name: "empty owner", repository: "/repo", wantErr: true},
		{name: "empty name", repository: "owner/", wantErr: true},
		{name: "empty", repository: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name, err := splitRepo(tt.repository)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "expected owner/repo")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestPluralize(t *testing.T) {
	assert.Equal(t, "comments", pluralize("comment", 0))
	assert.Equal(t, "comment", pluralize("comment", 1))
	assert.Equal(t, "comments", pluralize("comment", 2))
}
