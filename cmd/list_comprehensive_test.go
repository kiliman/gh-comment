package cmd

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/silouanwright/gh-comment/internal/github"
)

func TestRunListComprehensive(t *testing.T) {
	// Save original state
	originalClient := listClient
	originalVerbose := verbose
	originalQuiet := quiet
	originalAuthor := author
	originalFilter := filter

	originalHideAuthors := hideAuthors
	originalSince := since
	originalUntil := until

	originalListType := listType

	defer func() {
		listClient = originalClient
		verbose = originalVerbose
		quiet = originalQuiet
		author = originalAuthor
		filter = originalFilter

		hideAuthors = originalHideAuthors
		since = originalSince
		until = originalUntil

		listType = originalListType
	}()

	tests := []struct {
		name           string
		args           []string
		setupFlags     func()
		setupClient    func() github.GitHubAPI
		setupEnv       func()
		cleanupEnv     func()
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "client initialization with real client",
			args: []string{"123"},
			setupFlags: func() {
				verbose = false
				quiet = false
			},
			setupClient: func() github.GitHubAPI {
				listClient = nil // Force client creation
				return nil
			},
			setupEnv: func() {
				// Don't set any environment that would help client creation
			},
			cleanupEnv: func() {},
			wantErr:    true, // Real GitHub API will return 404 for test repo
		},
		{
			name: "verbose mode output",
			args: []string{"123"},
			setupFlags: func() {
				verbose = true
				quiet = false
				filter = ""
				showRecent = false

				hideAuthors = false
				author = "testuser"
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{
					issueComments: []github.Comment{
						{
							ID:        1,
							Body:      "Test comment",
							Type:      "issue",
							User:      github.User{Login: "testuser"},
							CreatedAt: time.Now(),
						},
					},
				}
				listClient = mockClient
				return mockClient
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			wantErr:    false,
		},
		{
			name: "invalid PR number argument",
			args: []string{"not-a-number"},
			setupFlags: func() {
				verbose = false
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "must be a valid integer",
		},
		// Removed test that calls real gh CLI - covered by integration tests
		{
			name: "API error from ListIssueComments",
			args: []string{"123"},
			setupFlags: func() {
				verbose = false
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{
					shouldErrorOnIssue: true,
				}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "mock issue error",
		},
		{
			name: "API error from ListReviewComments",
			args: []string{"123"},
			setupFlags: func() {
				verbose = false
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{
					shouldErrorOnReview: true,
				}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "mock review error",
		},
		{
			name: "date filter parsing error (since)",
			args: []string{"123"},
			setupFlags: func() {
				since = "invalid-date"
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "invalid since date",
		},
		{
			name: "date filter parsing error (until)",
			args: []string{"123"},
			setupFlags: func() {
				since = ""
				until = "invalid-date"
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "invalid until date",
		},
		{
			name: "status filter validation error",
			args: []string{"123"},
			setupFlags: func() {
				since = ""
				until = ""
				filter = "invalid-filter"
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "invalid filter",
		},
		{
			name: "type filter validation error",
			args: []string{"123"},
			setupFlags: func() {
				filter = ""
				listType = "invalid-type"
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{}
				listClient = mockClient
				return mockClient
			},
			setupEnv:       func() {},
			cleanupEnv:     func() {},
			wantErr:        true,
			expectedErrMsg: "invalid type",
		},
		{
			name: "no comments found",
			args: []string{"123"},
			setupFlags: func() {
				listType = ""
				verbose = false
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{
					issueComments:  []github.Comment{},
					reviewComments: []github.Comment{},
				}
				listClient = mockClient
				return mockClient
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			wantErr:    false,
		},
		{
			name: "successful execution with all comment types",
			args: []string{"123"},
			setupFlags: func() {
				verbose = false
				quiet = false
				hideAuthors = false
			},
			setupClient: func() github.GitHubAPI {
				mockClient := &MockGitHubClientForList{
					issueComments: []github.Comment{
						{
							ID:        1,
							Body:      "General comment",
							Type:      "issue",
							User:      github.User{Login: "user1"},
							CreatedAt: time.Now(),
						},
					},
					reviewComments: []github.Comment{
						{
							ID:        2,
							Body:      "Line-specific comment",
							Type:      "review",
							User:      github.User{Login: "user2"},
							CreatedAt: time.Now(),
							Path:      "test.go",
							Line:      42,
						},
					},
				}
				listClient = mockClient
				return mockClient
			},
			setupEnv:   func() {},
			cleanupEnv: func() {},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tt.setupFlags()
			tt.setupClient()
			tt.setupEnv()
			defer tt.cleanupEnv()

			// Execute the runList function directly since it uses fmt.Printf
			// which bypasses cobra's output redirection
			err := runList(nil, tt.args)

			// Verify results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				// For successful cases, we just verify no error occurred
				// Output testing would require stdout redirection which is complex in tests
				// The key functionality (fetching and filtering comments) is already tested
			}
		})
	}
}

// MockGitHubClientForList implements GitHubAPI for comprehensive list testing
type MockGitHubClientForList struct {
	issueComments       []github.Comment
	reviewComments      []github.Comment
	shouldErrorOnIssue  bool
	shouldErrorOnReview bool
}

func (m *MockGitHubClientForList) ListIssueComments(owner, repo string, prNumber int) ([]github.Comment, error) {
	if m.shouldErrorOnIssue {
		return nil, errors.New("mock issue error")
	}
	return m.issueComments, nil
}

func (m *MockGitHubClientForList) ListReviewComments(owner, repo string, prNumber int) ([]github.Comment, error) {
	if m.shouldErrorOnReview {
		return nil, errors.New("mock review error")
	}
	return m.reviewComments, nil
}

// Implement other required methods with no-ops
func (m *MockGitHubClientForList) CreateIssueComment(owner, repo string, prNumber int, body string) (*github.Comment, error) {
	return nil, nil
}

func (m *MockGitHubClientForList) CreateReviewCommentReply(owner, repo string, commentID int, body string) (*github.Comment, error) {
	return nil, nil
}

func (m *MockGitHubClientForList) FindReviewThreadForComment(owner, repo string, prNumber, commentID int) (string, error) {
	return "", nil
}

func (m *MockGitHubClientForList) ResolveReviewThread(threadID string) error {
	return nil
}

func (m *MockGitHubClientForList) AddReaction(owner, repo string, commentID int, prNumber int, reaction string) error {
	return nil
}

func (m *MockGitHubClientForList) RemoveReaction(owner, repo string, commentID int, prNumber int, reaction string) error {
	return nil
}

func (m *MockGitHubClientForList) EditComment(owner, repo string, commentID int, prNumber int, body string) error {
	return nil
}

func (m *MockGitHubClientForList) AddReviewComment(owner, repo string, pr int, comment github.ReviewCommentInput) error {
	return nil
}

func (m *MockGitHubClientForList) FetchPRDiff(owner, repo string, pr int) (*github.PullRequestDiff, error) {
	return nil, nil
}

func (m *MockGitHubClientForList) CreateReview(owner, repo string, pr int, review github.ReviewInput) error {
	return nil
}

func (m *MockGitHubClientForList) GetPRDetails(owner, repo string, pr int) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockGitHubClientForList) FetchFileContent(owner, repo, path, ref string) (string, error) {
	return "", nil
}

func (m *MockGitHubClientForList) FindPendingReview(owner, repo string, pr int) (int, error) {
	return 0, nil
}

func (m *MockGitHubClientForList) SubmitReview(owner, repo string, pr, reviewID int, body, event string) error {
	return nil
}
