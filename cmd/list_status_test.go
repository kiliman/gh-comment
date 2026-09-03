package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/silouanwright/gh-comment/internal/github"
)

// withListStatus sets the --status global for one test.
//
// It also neutralizes the other filter globals. These are package-level and
// other tests in this package leave values behind (a stray filter="all" made
// validateAndParseFilters fail here for reasons that had nothing to do with
// --status), so pinning them is what keeps this test about one thing.
func withListStatus(t *testing.T, status string) {
	t.Helper()
	originalStatus, originalFilter, originalRecent := listStatus, filter, showRecent
	listStatus, filter, showRecent = status, "", false
	t.Cleanup(func() {
		listStatus, filter, showRecent = originalStatus, originalFilter, originalRecent
	})
}

func TestFilterCommentsByStatus(t *testing.T) {
	comments := []Comment{
		{ID: 1, Type: "review", Resolved: false},
		{ID: 2, Type: "review", Resolved: true},
		{ID: 3, Type: "issue", Resolved: false}, // issue comments have no thread
	}

	// Neutralize the other filters so only --status is under test.
	originals := []interface{}{author, listType, sinceTime, untilTime}
	author, listType, sinceTime, untilTime = "", "", nil, nil
	t.Cleanup(func() {
		author = originals[0].(string)
		listType = originals[1].(string)
	})

	t.Run("all keeps everything", func(t *testing.T) {
		withListStatus(t, "all")
		assert.Len(t, filterComments(comments), 3)
	})

	t.Run("open drops resolved review comments but keeps issue comments", func(t *testing.T) {
		// An issue comment can never be resolved, so it is always open —
		// narrowing to threads is what --type review is for.
		withListStatus(t, "open")

		got := filterComments(comments)

		require.Len(t, got, 2)
		assert.Equal(t, 1, got[0].ID)
		assert.Equal(t, 3, got[1].ID, "the issue comment counts as open")
	})

	t.Run("resolved keeps only resolved review comments", func(t *testing.T) {
		withListStatus(t, "resolved")

		got := filterComments(comments)

		require.Len(t, got, 1)
		assert.Equal(t, 2, got[0].ID)
		assert.NotContains(t, []int{got[0].ID}, 3, "an issue comment is never resolved")
	})
}

func TestValidateStatusFlag(t *testing.T) {
	for _, status := range []string{"", "open", "resolved", "all"} {
		t.Run("accepts "+status, func(t *testing.T) {
			withListStatus(t, status)
			assert.NoError(t, validateAndParseFilters())
		})
	}

	t.Run("rejects anything else", func(t *testing.T) {
		withListStatus(t, "unresolved")
		err := validateAndParseFilters()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
		assert.Contains(t, err.Error(), "open, resolved, all")
	})
}

func TestFetchAllCommentsResolutionLookup(t *testing.T) {
	t.Run("skips the extra query when status is all", func(t *testing.T) {
		// The default listing must not pay for a GraphQL round-trip it does
		// not use.
		withListStatus(t, "all")
		client := &threadCountingClient{MockClient: github.NewMockClient()}

		_, err := fetchAllComments(client, "owner/repo", 123)

		require.NoError(t, err)
		assert.Zero(t, client.threadCalls, "no filter asked for resolution state")
	})

	t.Run("marks comments from their thread when status filters", func(t *testing.T) {
		withListStatus(t, "open")
		client := github.NewMockClient()
		// MockClient's single review comment is ID 654321.
		client.ReviewThreads = []github.ReviewThread{
			{ID: "RT_1", IsResolved: true, CommentIDs: []int{654321}},
		}

		got, err := fetchAllComments(client, "owner/repo", 123)

		require.NoError(t, err)
		for _, c := range got {
			if c.ID == 654321 {
				assert.True(t, c.Resolved, "should inherit its thread's resolved state")
			}
		}
	})

	t.Run("a comment in no known thread reads as open", func(t *testing.T) {
		withListStatus(t, "open")
		client := github.NewMockClient()
		client.ReviewThreads = nil // PR has no threads at all

		got, err := fetchAllComments(client, "owner/repo", 123)

		require.NoError(t, err)
		for _, c := range got {
			assert.False(t, c.Resolved, "absence of a thread must not read as resolved")
		}
	})

	t.Run("surfaces a thread lookup failure", func(t *testing.T) {
		withListStatus(t, "open")
		client := github.NewMockClient()
		client.ListReviewThreadsError = fmt.Errorf("GraphQL unavailable")

		_, err := fetchAllComments(client, "owner/repo", 123)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "--status")
	})
}

type threadCountingClient struct {
	*github.MockClient
	threadCalls int
}

func (c *threadCountingClient) ListReviewThreads(owner, repo string, pr int) ([]github.ReviewThread, error) {
	c.threadCalls++
	return c.MockClient.ListReviewThreads(owner, repo, pr)
}
