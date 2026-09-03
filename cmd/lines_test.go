package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/silouanwright/gh-comment/internal/github"
)

func TestRunLinesWithMockClient(t *testing.T) {
	// Save original state
	originalClient := linesClient
	originalRepo := repo
	defer func() {
		linesClient = originalClient
		repo = originalRepo
	}()

	tests := []struct {
		name           string
		args           []string
		setupMock      func(*github.MockClient)
		setupRepo      string
		wantErr        bool
		expectedErrMsg string
		checkOutput    func(string) bool
	}{
		{
			name: "show commentable lines for existing file",
			args: []string{"123", "main.go"},
			setupMock: func(mock *github.MockClient) {
				// MockClient already returns test.go with lines 42, 43
			},
			setupRepo: "owner/repo",
			wantErr:   false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "❌ File 'main.go' not found in PR #123 diff") &&
					strings.Contains(output, "Available files in this PR:") &&
					strings.Contains(output, "• test.go")
			},
		},
		{
			name: "show commentable lines for test.go (existing in mock)",
			args: []string{"123", "test.go"},
			setupMock: func(mock *github.MockClient) {
				// MockClient returns test.go with lines 42, 43
			},
			setupRepo: "owner/repo",
			wantErr:   false,
			checkOutput: func(output string) bool {
				return strings.Contains(output, "✅ Commentable lines in test.go (PR #123)") &&
					strings.Contains(output, "42") &&
					strings.Contains(output, "43")
			},
		},
		{
			name:           "invalid PR number",
			args:           []string{"abc", "main.go"},
			setupMock:      func(mock *github.MockClient) {},
			setupRepo:      "owner/repo",
			wantErr:        true,
			expectedErrMsg: "invalid PR number 'abc': must be a valid integer",
		},
		{
			name:           "invalid repository format",
			args:           []string{"123", "main.go"},
			setupMock:      func(mock *github.MockClient) {},
			setupRepo:      "invalid-repo",
			wantErr:        true,
			expectedErrMsg: "invalid repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock client
			mockClient := github.NewMockClient()
			tt.setupMock(mockClient)
			linesClient = mockClient

			// Setup repository
			repo = tt.setupRepo

			// Run command
			err := runLines(nil, tt.args)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLinesCommandDryRun(t *testing.T) {
	// Save original state
	originalClient := linesClient
	originalRepo := repo
	originalDryRun := dryRun
	defer func() {
		linesClient = originalClient
		repo = originalRepo
		dryRun = originalDryRun
	}()

	// Setup
	mockClient := github.NewMockClient()
	linesClient = mockClient
	repo = "owner/repo"
	dryRun = true

	// Run command
	err := runLines(nil, []string{"123", "main.go"})

	// Check results
	assert.NoError(t, err)
}

func TestLinesCommandVerbose(t *testing.T) {
	// Save original state
	originalClient := linesClient
	originalRepo := repo
	originalVerbose := verbose
	defer func() {
		linesClient = originalClient
		repo = originalRepo
		verbose = originalVerbose
	}()

	// Setup
	mockClient := github.NewMockClient()
	linesClient = mockClient
	repo = "owner/repo"
	verbose = true

	// Run command
	err := runLines(nil, []string{"123", "test.go"})

	// Check results
	assert.NoError(t, err)
}

func TestLinesCommandShowCodeUsesRealFileContent(t *testing.T) {
	originalClient := linesClient
	originalRepo := repo
	originalShowCode := showCodeContext
	defer func() {
		linesClient = originalClient
		repo = originalRepo
		showCodeContext = originalShowCode
	}()

	mockClient := github.NewMockClient()
	linesClient = mockClient
	repo = "owner/repo"
	showCodeContext = true

	output := captureOutput(func() {
		err := runLines(nil, []string{"123", "test.go"})
		assert.NoError(t, err)
	})

	assert.Contains(t, output, "42: println(\"hello\")")
	assert.Contains(t, output, "43: }")
	assert.NotContains(t, output, "[code content would be shown here]")
}

func TestLinesCommandWithClientInitialization(t *testing.T) {
	// Save original state
	originalClient := linesClient
	originalRepo := repo
	defer func() {
		linesClient = originalClient
		repo = originalRepo
	}()

	// Set up mock environment to prevent real API calls
	originalMockURL := os.Getenv("MOCK_SERVER_URL")
	os.Setenv("MOCK_SERVER_URL", "http://localhost:8080")
	defer os.Setenv("MOCK_SERVER_URL", originalMockURL)

	// Clear client to test initialization
	linesClient = nil
	repo = "owner/repo"

	// Run command - this will initialize the client with mock
	runLines(nil, []string{"123", "test.go"})

	// Should have initialized the client (even if operation fails due to mock)
	assert.NotNil(t, linesClient) // Client should have been initialized
	// Error is expected since we're using mock client
}

func TestGroupConsecutiveLines(t *testing.T) {
	tests := []struct {
		name     string
		lines    []int
		expected []lineRange
	}{
		{
			name:     "empty slice",
			lines:    []int{},
			expected: nil,
		},
		{
			name:     "single line",
			lines:    []int{42},
			expected: []lineRange{{start: 42, end: 42}},
		},
		{
			name:     "consecutive lines",
			lines:    []int{10, 11, 12, 13},
			expected: []lineRange{{start: 10, end: 13}},
		},
		{
			name:     "non-consecutive lines",
			lines:    []int{10, 15, 20},
			expected: []lineRange{{start: 10, end: 10}, {start: 15, end: 15}, {start: 20, end: 20}},
		},
		{
			name:     "mixed consecutive and non-consecutive",
			lines:    []int{10, 11, 12, 15, 16, 20},
			expected: []lineRange{{start: 10, end: 12}, {start: 15, end: 16}, {start: 20, end: 20}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupConsecutiveLines(tt.lines)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintCommentAnchors(t *testing.T) {
	// The mock file content is "// line N" for every line, except line 42
	// which is `println("hello")` and line 43 which is `}`.
	t.Run("echoes the line each comment attached to", func(t *testing.T) {
		client := github.NewMockClient()

		output := captureOutput(func() {
			printCommentAnchors(client, "owner", "repo", 123, []github.ReviewCommentInput{
				{Path: "main.go", Line: 42, Body: "x"},
			})
		})

		assert.Contains(t, output, `main.go:42`)
		assert.Contains(t, output, `println(\"hello\")`,
			"the anchor line is quoted, so a wrong line is visible without opening the PR")
	})

	t.Run("marks a range comment as elided from its start line", func(t *testing.T) {
		client := github.NewMockClient()

		output := captureOutput(func() {
			printCommentAnchors(client, "owner", "repo", 123, []github.ReviewCommentInput{
				{Path: "main.go", StartLine: 42, Line: 43, Body: "x"},
			})
		})

		assert.Contains(t, output, "main.go:42", "a range anchors on its start line")
		assert.Contains(t, output, "…")
	})

	t.Run("fetches each distinct file once, not once per comment", func(t *testing.T) {
		client := &countingFileClient{MockClient: github.NewMockClient()}

		captureOutput(func() {
			printCommentAnchors(client, "owner", "repo", 123, []github.ReviewCommentInput{
				{Path: "main.go", Line: 42},
				{Path: "main.go", Line: 43},
				{Path: "other.go", Line: 42},
			})
		})

		assert.Equal(t, 2, client.fetches, "three comments across two files means two fetches")
	})

	t.Run("stays quiet when the fetch fails", func(t *testing.T) {
		// The review is already created by now. A display nicety must never
		// turn a successful review into a non-zero exit.
		client := &failingFileClient{MockClient: github.NewMockClient()}

		output := captureOutput(func() {
			printCommentAnchors(client, "owner", "repo", 123, []github.ReviewCommentInput{
				{Path: "main.go", Line: 42},
			})
		})

		assert.Empty(t, output)
	})

	t.Run("no comments means no output and no calls", func(t *testing.T) {
		client := &countingFileClient{MockClient: github.NewMockClient()}

		output := captureOutput(func() {
			printCommentAnchors(client, "owner", "repo", 123, nil)
		})

		assert.Empty(t, output)
		assert.Zero(t, client.fetches)
	})
}

type countingFileClient struct {
	*github.MockClient
	fetches int
}

func (c *countingFileClient) FetchFileContent(owner, repo, path, ref string) (string, error) {
	c.fetches++
	return c.MockClient.FetchFileContent(owner, repo, path, ref)
}

type failingFileClient struct {
	*github.MockClient
}

func (c *failingFileClient) FetchFileContent(owner, repo, path, ref string) (string, error) {
	return "", fmt.Errorf("file not found in this ref")
}
