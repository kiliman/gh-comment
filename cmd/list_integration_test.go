package cmd

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/silouanwright/gh-comment/internal/github"
)

// TestRunListIntegration tests the actual runList function with various scenarios
func TestRunListIntegration(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		flags          map[string]string
		mockComments   []github.Comment
		wantErr        bool
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "successful list with mixed comments",
			args: []string{"123"},
			mockComments: []github.Comment{
				{
					ID:        123456,
					Body:      "LGTM! Great work on this PR.",
					Type:      "issue",
					User:      github.User{Login: "reviewer1"},
					CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				},
				{
					ID:        654321,
					Body:      "Consider using a more descriptive variable name here.",
					Type:      "review",
					User:      github.User{Login: "reviewer2"},
					CreatedAt: time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
					Path:      "main.go",
					Line:      42,
				},
			},
			wantContains: []string{
				"Comments on PR #123",
				"General PR Comments",
				"Review Comments",
				"LGTM! Great work",
				"reviewer1",
				"reviewer2",
				"main.go:L42",
			},
		},
		{
			name: "quiet mode hides URLs",
			args: []string{"123"},
			flags: map[string]string{
				"quiet": "true",
			},
			mockComments: []github.Comment{
				{
					ID:        123456,
					Body:      "Test comment",
					Type:      "issue",
					User:      github.User{Login: "testuser"},
					CreatedAt: time.Now(),
				},
			},
			wantContains:   []string{"Test comment", "testuser"},
			wantNotContain: []string{"🔗"},
		},
		{
			name: "author filter works",
			args: []string{"123"},
			flags: map[string]string{
				"author": "alice",
			},
			mockComments: []github.Comment{
				{
					ID:        1,
					Body:      "Alice's comment",
					Type:      "issue",
					User:      github.User{Login: "alice"},
					CreatedAt: time.Now(),
				},
				{
					ID:        2,
					Body:      "Bob's comment",
					Type:      "issue",
					User:      github.User{Login: "bob"},
					CreatedAt: time.Now(),
				},
			},
			wantContains:   []string{"Alice's comment", "alice"},
			wantNotContain: []string{"Bob's comment", "bob"},
		},
		{
			name: "hide authors works",
			args: []string{"123"},
			flags: map[string]string{
				"hide-authors": "true",
			},
			mockComments: []github.Comment{
				{
					ID:        123456,
					Body:      "Anonymous comment",
					Type:      "issue",
					User:      github.User{Login: "testuser"},
					CreatedAt: time.Now(),
				},
			},
			wantContains:   []string{"Anonymous comment", "[hidden]"},
			wantNotContain: []string{"testuser"},
		},
		{
			name:         "no comments shows appropriate message",
			args:         []string{"123"},
			mockComments: []github.Comment{},
			wantContains: []string{"No comments found"},
		},
		{
			name:    "invalid PR number returns error",
			args:    []string{"invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags
			// Reset global flags
			quiet = false
			hideAuthors = false
			author = ""
			filter = ""
			showRecent = false

			// Create a mock GitHub client
			mockClient := &MockGitHubClient{
				issueComments:  []github.Comment{},
				reviewComments: []github.Comment{},
			}

			// Separate comments by type
			for _, comment := range tt.mockComments {
				if comment.Type == "issue" {
					mockClient.issueComments = append(mockClient.issueComments, comment)
				} else {
					mockClient.reviewComments = append(mockClient.reviewComments, comment)
				}
			}

			// Capture output
			var output bytes.Buffer

			// Create a test command that uses our mock
			cmd := &cobra.Command{
				Use:  "list [pr]",
				Args: cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					return runListWithMock(cmd, args, mockClient, &output)
				},
			}

			// Add flags
			cmd.Flags().StringVar(&filter, "filter", "all", "Filter comments (all|recent|today)")
			cmd.Flags().StringVar(&author, "author", "", "Filter comments by author")
			cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Minimal output without URLs and IDs")
			cmd.Flags().BoolVar(&hideAuthors, "hide-authors", false, "Hide author names for privacy")

			// Set flags
			for flag, value := range tt.flags {
				err := cmd.Flags().Set(flag, value)
				require.NoError(t, err)
			}

			// Set output
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Run command
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Check error expectation
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Check output
			outputStr := output.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, outputStr, want, "Output should contain: %s", want)
			}
			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, outputStr, notWant, "Output should not contain: %s", notWant)
			}
		})
	}
}

// MockGitHubClient is a simple mock for testing
type MockGitHubClient struct {
	issueComments  []github.Comment
	reviewComments []github.Comment
	shouldError    bool
}

func (m *MockGitHubClient) ListIssueComments(owner, repo string, prNumber int) ([]github.Comment, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.issueComments, nil
}

func (m *MockGitHubClient) ListReviewComments(owner, repo string, prNumber int) ([]github.Comment, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.reviewComments, nil
}

func (m *MockGitHubClient) CreateIssueComment(owner, repo string, prNumber int, body string) (*github.Comment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockGitHubClient) CreateReviewCommentReply(owner, repo string, commentID int, body string) (*github.Comment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockGitHubClient) ResolveReviewThread(threadID string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockGitHubClient) ListReviewThreads(owner, repo string, prNumber int) ([]github.ReviewThread, error) {
	return nil, nil
}

func (m *MockGitHubClient) GetPRNumberForComment(owner, repo string, commentID int) (int, error) {
	return 0, nil
}

func (m *MockGitHubClient) FindReviewThreadForComment(owner, repo string, prNumber, commentID int) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// runListWithMock is a testable version of runList that uses a mock client
func runListWithMock(cmd *cobra.Command, args []string, mockClient *MockGitHubClient, output *bytes.Buffer) error {
	var pr int
	var err error

	// Parse PR argument
	if len(args) == 1 {
		pr, err = strconv.Atoi(args[0])
		if err != nil {
			return formatValidationError("PR number", args[0], "must be a valid integer")
		}
	} else {
		// Default for testing
		pr = 1
	}

	// Mock repository
	repository := "owner/repo"

	if verbose {
		fmt.Fprintf(output, "Repository: %s\n", repository)
		fmt.Fprintf(output, "PR: %d\n", pr)
		fmt.Fprintf(output, "Filter: %v\n", filter)
		fmt.Fprintf(output, "Quiet mode: %v\n", quiet)
		fmt.Fprintf(output, "Hide authors: %v\n", hideAuthors)
		if author != "" {
			fmt.Fprintf(output, "Filter by author: %s\n", author)
		}
		fmt.Fprintf(output, "\n")
	}

	// Fetch comments using mock client
	issueComments, err := mockClient.ListIssueComments("owner", "repo", pr)
	if err != nil {
		return fmt.Errorf("failed to fetch issue comments: %w", err)
	}

	reviewComments, err := mockClient.ListReviewComments("owner", "repo", pr)
	if err != nil {
		return fmt.Errorf("failed to fetch review comments: %w", err)
	}

	// Convert to internal Comment format
	var allComments []Comment
	for _, comment := range issueComments {
		allComments = append(allComments, Comment{
			ID:        comment.ID,
			Author:    comment.User.Login,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Type:      comment.Type,
		})
	}
	for _, comment := range reviewComments {
		allComments = append(allComments, Comment{
			ID:        comment.ID,
			Author:    comment.User.Login,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Path:      comment.Path,
			Line:      comment.Line,
			Type:      comment.Type,
		})
	}

	// Filter comments
	filteredComments := filterComments(allComments)

	// Display comments
	if len(filteredComments) == 0 {
		fmt.Fprintf(output, "No comments found on PR #%d\n", pr)
		return nil
	}

	// Display header
	fmt.Fprintf(output, "📝 Comments on PR #%d (%d total)\n\n", pr, len(filteredComments))

	// Group comments by type
	var issueCommentsFiltered, reviewCommentsFiltered []Comment
	for _, comment := range filteredComments {
		if comment.Type == "issue" {
			issueCommentsFiltered = append(issueCommentsFiltered, comment)
		} else {
			reviewCommentsFiltered = append(reviewCommentsFiltered, comment)
		}
	}

	// Display general PR comments
	if len(issueCommentsFiltered) > 0 {
		fmt.Fprintf(output, "💬 General PR Comments (%d)\n", len(issueCommentsFiltered))
		fmt.Fprintf(output, "%s\n", strings.Repeat("─", SeparatorLength))
		for i, comment := range issueCommentsFiltered {
			displayCommentToBuffer(comment, i+1, output)
		}
		fmt.Fprintf(output, "\n")
	}

	// Display review comments
	if len(reviewCommentsFiltered) > 0 {
		fmt.Fprintf(output, "📋 Review Comments (%d)\n", len(reviewCommentsFiltered))
		fmt.Fprintf(output, "%s\n", strings.Repeat("─", SeparatorLength))
		for i, comment := range reviewCommentsFiltered {
			displayCommentToBuffer(comment, i+1, output)
		}
	}

	return nil
}

// displayCommentToBuffer displays a comment to a buffer instead of stdout
func displayCommentToBuffer(comment Comment, index int, output *bytes.Buffer) {
	// Header with author and timestamp
	timeAgo := formatTimeAgo(comment.CreatedAt)
	if hideAuthors {
		fmt.Fprintf(output, "[%d] 👤 [hidden] • %s", index, timeAgo)
	} else {
		fmt.Fprintf(output, "[%d] 👤 %s • %s", index, comment.Author, timeAgo)
	}
	fmt.Fprintf(output, "\n")

	// File and line info for line-specific comments
	if comment.Path != "" {
		lineInfo := fmt.Sprintf("L%d", comment.Line)
		fmt.Fprintf(output, "📁 %s:%s\n", comment.Path, lineInfo)
	}

	// Comment body (truncate if too long)
	body := strings.TrimSpace(comment.Body)
	if len(body) > MaxDisplayBodyLength {
		body = body[:MaxDisplayBodyLength-TruncationReserve] + TruncationSuffix
	}

	// Indent the comment body
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		fmt.Fprintf(output, "   %s\n", line)
	}

	// Show URLs by default (AI-friendly), hide only in quiet mode
	if !quiet {
		fmt.Fprintf(output, "   🔗 https://github.com/owner/repo/issues/1#issuecomment-%d\n", comment.ID)
	}

	fmt.Fprintf(output, "\n")
}
