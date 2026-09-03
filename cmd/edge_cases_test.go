package cmd

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/silouanwright/gh-comment/internal/github"
)

// TestEdgeCasesAndCornerCases tests various edge cases to improve coverage
func TestEdgeCasesAndCornerCases(t *testing.T) {
	// Save original state
	originalRepo := repo
	originalPRNumber := prNumber
	originalVerbose := verbose
	originalDryRun := dryRun
	defer func() {
		repo = originalRepo
		prNumber = originalPRNumber
		verbose = originalVerbose
		dryRun = originalDryRun
	}()

	t.Run("formatTimeAgo edge cases", func(t *testing.T) {
		now := time.Now()

		// Test just now
		justNow := now.Add(-30 * time.Second)
		result := formatTimeAgo(justNow)
		assert.Equal(t, "just now", result)

		// Test exactly 1 minute
		oneMinute := now.Add(-1 * time.Minute)
		result = formatTimeAgo(oneMinute)
		assert.Equal(t, "1 minute ago", result)

		// Test exactly 1 hour
		oneHour := now.Add(-1 * time.Hour)
		result = formatTimeAgo(oneHour)
		assert.Equal(t, "1 hour ago", result)

		// Test exactly 1 day
		oneDay := now.Add(-24 * time.Hour)
		result = formatTimeAgo(oneDay)
		assert.Equal(t, "1 day ago", result)

		// Test old date (more than 7 days)
		oldDate := now.Add(-10 * 24 * time.Hour)
		result = formatTimeAgo(oldDate)
		assert.Contains(t, result, "2") // Should contain date format
	})

	t.Run("displayDiffHunk edge cases", func(t *testing.T) {
		// Test empty diff hunk
		assert.NotPanics(t, func() {
			displayDiffHunk("")
		})

		// Test diff hunk with only whitespace
		assert.NotPanics(t, func() {
			displayDiffHunk("   \n   \n")
		})

		// Test diff hunk with header lines
		diffHunk := `@@ -1,4 +1,5 @@
 context line
+added line
-removed line
 another context`
		assert.NotPanics(t, func() {
			displayDiffHunk(diffHunk)
		})
	})

	t.Run("matchesAuthorFilter edge cases", func(t *testing.T) {
		// Test empty filter (should match all)
		assert.True(t, matchesAuthorFilter("anyone", ""))

		// Test empty author
		assert.False(t, matchesAuthorFilter("", "filter"))

		// Test both empty
		assert.True(t, matchesAuthorFilter("", ""))

		// Test exact match
		assert.True(t, matchesAuthorFilter("user", "user"))

		// Test wildcard at beginning
		assert.True(t, matchesAuthorFilter("prefix-user", "*user"))

		// Test wildcard at end
		assert.True(t, matchesAuthorFilter("user-suffix", "user*"))

		// Test wildcard in middle
		assert.True(t, matchesAuthorFilter("pre-middle-suf", "pre*suf"))

		// Test case insensitive
		assert.True(t, matchesAuthorFilter("User", "user"))
		assert.True(t, matchesAuthorFilter("user", "User"))

		// Test no match
		assert.False(t, matchesAuthorFilter("alice", "bob"))

		// Test invalid regex (shouldn't crash)
		assert.NotPanics(t, func() {
			matchesAuthorFilter("user", "invalid[regex")
		})
	})

	t.Run("parseFlexibleDate with dateparse edge cases", func(t *testing.T) {
		// Test various formats that dateparse should handle
		tests := []struct {
			input    string
			expected bool
		}{
			{"1 second ago", true},
			{"1 seconds ago", true},
			{"1 minute ago", true},
			{"1 minutes ago", true},
			{"1 hour ago", true},
			{"1 hours ago", true},
			{"1 day ago", true},
			{"1 days ago", true},
			{"1 week ago", true},
			{"1 weeks ago", true},
			{"1 month ago", true},
			{"1 months ago", true},
			{"1 year ago", true},
			{"1 years ago", true},
			{"2024-01-01", true},
			{"Jan 1, 2024", true},
			{"", false},
			{"definitely not a date", false},
		}

		for _, tc := range tests {
			_, err := parseFlexibleDate(tc.input)
			if tc.expected {
				assert.NoError(t, err, "Should parse: %s", tc.input)
			} else {
				assert.Error(t, err, "Should fail to parse: %s", tc.input)
			}
		}
	})

	t.Run("parseFlexibleDate edge cases", func(t *testing.T) {
		// Test various formats
		formats := []struct {
			input    string
			expected bool
		}{
			{"2024-01-01", true},
			{"2024-01-01 12:00:00", true},
			{"01/01/2024", true},
			{"Jan 1, 2024", true},
			{"January 1, 2024", true},
			{"2024-01-01T12:00:00Z", true},
			{"1 day ago", true},
			{"invalid date", false},
			{"", false},
			{"2024-13-45", false}, // Invalid date
		}

		for _, tc := range formats {
			_, err := parseFlexibleDate(tc.input)
			if tc.expected {
				assert.NoError(t, err, "Should parse: %s", tc.input)
			} else {
				assert.Error(t, err, "Should fail to parse: %s", tc.input)
			}
		}
	})

	t.Run("containsString edge cases", func(t *testing.T) {
		// Test empty slice
		assert.False(t, containsString([]string{}, "item"))

		// Test nil slice
		assert.False(t, containsString(nil, "item"))

		// Test empty string
		assert.True(t, containsString([]string{""}, ""))
		assert.False(t, containsString([]string{"a", "b"}, ""))

		// Test duplicates
		assert.True(t, containsString([]string{"a", "a", "b"}, "a"))

		// Test large slice
		largeSlice := make([]string, 1000)
		for i := range largeSlice {
			largeSlice[i] = "item"
		}
		assert.True(t, containsString(largeSlice, "item"))
		assert.False(t, containsString(largeSlice, "notfound"))
	})

	t.Run("getCurrentRepo edge cases", func(t *testing.T) {
		// Test with repo already set (no API call)
		repo = "preset/repo"
		result, err := getCurrentRepo()
		assert.NoError(t, err)
		assert.Equal(t, "preset/repo", result)

		// Skip testing with empty repo to avoid real API calls
		// This would be covered by integration tests
	})

	t.Run("getPRContext edge cases", func(t *testing.T) {
		// Test with PR already set (no API call)
		repo = "preset/repo"
		prNumber = 123
		gotRepo, result, err := getPRContext()
		assert.NoError(t, err)
		assert.Equal(t, 123, result)
		assert.Equal(t, "preset/repo", gotRepo)

		// A non-positive --pr is not a valid override, so it falls through to
		// branch detection rather than being passed along as a PR number.
		prNumber = -1
		stubGhExec(t, "77\n", nil)
		_, result, err = getPRContext()
		assert.NoError(t, err)
		assert.Equal(t, 77, result)
	})

	t.Run("validateReaction edge cases", func(t *testing.T) {
		// Test all valid reactions
		validReactions := []string{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"}
		for _, reaction := range validReactions {
			assert.True(t, validateReaction(reaction), "Should be valid: %s", reaction)
		}

		// Test invalid reactions
		invalidReactions := []string{"", "invalid", "thumbsup", "thumbsdown", "LAUGH", "+2", "smile"}
		for _, reaction := range invalidReactions {
			assert.False(t, validateReaction(reaction), "Should be invalid: %s", reaction)
		}
	})
}

func TestGlobalVariableEdgeCases(t *testing.T) {
	// Save original state
	originalValues := map[string]interface{}{
		"repo":         repo,
		"prNumber":     prNumber,
		"validateDiff": validateDiff,
		"dryRun":       dryRun,
		"verbose":      verbose,
	}

	defer func() {
		repo = originalValues["repo"].(string)
		prNumber = originalValues["prNumber"].(int)
		validateDiff = originalValues["validateDiff"].(bool)
		dryRun = originalValues["dryRun"].(bool)
		verbose = originalValues["verbose"].(bool)
	}()

	t.Run("global flag combinations", func(t *testing.T) {
		// Test various flag combinations that don't hit APIs
		combinations := []struct {
			name         string
			repo         string
			prNumber     int
			validateDiff bool
			dryRun       bool
			verbose      bool
		}{
			{"custom repo and PR", "owner/repo", 123, true, false, false},
			{"all enabled with preset values", "owner/repo", 123, true, true, true},
			{"preset values with dry run", "preset/repo", 456, false, true, false},
		}

		for _, combo := range combinations {
			t.Run(combo.name, func(t *testing.T) {
				repo = combo.repo
				prNumber = combo.prNumber
				validateDiff = combo.validateDiff
				dryRun = combo.dryRun
				verbose = combo.verbose

				// Test that these settings don't cause panics (no API calls)
				assert.NotPanics(t, func() {
					getCurrentRepo() // Will return preset repo
				})
				assert.NotPanics(t, func() {
					getPRContext() // Will return preset repo and PR number
				})
			})
		}
	})
}

func TestMockClientEdgeCases(t *testing.T) {
	t.Run("mock client with extreme configurations", func(t *testing.T) {
		client := github.NewMockClient()

		// Test with all errors enabled
		client.ListIssueCommentsError = errors.New("issue comments error")
		client.ListReviewCommentsError = errors.New("review comments error")
		client.CreateCommentError = errors.New("create comment error")
		client.ResolveThreadError = errors.New("resolve thread error")
		client.FindPendingReviewError = errors.New("find pending review error")
		client.SubmitReviewError = errors.New("submit review error")

		// Test that all methods return appropriate errors
		_, err := client.ListIssueComments("owner", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issue comments error")

		_, err = client.ListReviewComments("owner", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "review comments error")

		_, err = client.CreateIssueComment("owner", "repo", 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "create comment error")

		err = client.ResolveReviewThread("thread-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resolve thread error")

		_, err = client.FindPendingReview("owner", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "find pending review error")

		err = client.SubmitReview("owner", "repo", 123, 456, "body", "event")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "submit review error")
	})
}
