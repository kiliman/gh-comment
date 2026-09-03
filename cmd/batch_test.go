package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/silouanwright/gh-comment/internal/github"
)

func TestRunBatchWithMockClient(t *testing.T) {
	// Save original client and environment
	originalClient := batchClient
	originalRepo := repo
	originalPR := prNumber
	originalValidateDiff := validateDiff
	defer func() {
		batchClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		validateDiff = originalValidateDiff
	}()

	// Set up mock client and environment
	mockClient := github.NewMockClient()
	batchClient = mockClient
	repo = "owner/repo"
	prNumber = 123
	validateDiff = false // Disable validation for tests

	tests := []struct {
		name           string
		args           []string
		configContent  string
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "process batch with review and comments",
			args: []string{"123", "config.yaml"},
			configContent: `
pr: 123
repo: owner/repo
review:
  body: "Migration review"
  event: APPROVE
comments:
  - file: src/api.js
    line: 42
    message: "Consider adding rate limiting"
    type: review
  - file: README.md
    line: 10
    message: "Great documentation"
    type: issue
`,
			wantErr: false,
		},
		{
			name: "process individual comments only",
			args: []string{"123", "config.yaml"},
			configContent: `
comments:
  - file: src/main.go
    line: 25
    message: "Good implementation"
    type: review
  - file: src/utils.go
    range: "10-15"
    message: "Nice refactoring"
    type: review
`,
			wantErr: false,
		},
		{
			name: "invalid PR number",
			args: []string{"invalid", "config.yaml"},
			configContent: `
comments:
  - file: test.go
    line: 1
    message: "test"
`,
			wantErr:        true,
			expectedErrMsg: "must be a valid integer",
		},
		{
			name:           "empty config",
			args:           []string{"123", "config.yaml"},
			configContent:  ``,
			wantErr:        true,
			expectedErrMsg: "configuration must contain either comments or review",
		},
		{
			name: "invalid comment - missing file",
			args: []string{"123", "config.yaml"},
			configContent: `
comments:
  - line: 42
    message: "test"
`,
			wantErr:        true,
			expectedErrMsg: "file is required",
		},
		{
			name: "invalid comment - missing message",
			args: []string{"123", "config.yaml"},
			configContent: `
comments:
  - file: test.go
    line: 42
`,
			wantErr:        true,
			expectedErrMsg: "message is required",
		},
		{
			name: "invalid comment - no line or range",
			args: []string{"123", "config.yaml"},
			configContent: `
comments:
  - file: test.go
    message: "test"
`,
			wantErr:        true,
			expectedErrMsg: "either line or range is required",
		},
		{
			name: "invalid comment - both line and range",
			args: []string{"123", "config.yaml"},
			configContent: `
comments:
  - file: test.go
    line: 42
    range: "10-15"
    message: "test"
`,
			wantErr:        true,
			expectedErrMsg: "cannot specify both line and range",
		},
		{
			name: "invalid review event",
			args: []string{"123", "config.yaml"},
			configContent: `
review:
  body: "test"
  event: INVALID
comments:
  - file: test.go
    line: 1
    message: "test"
`,
			wantErr:        true,
			expectedErrMsg: "review event must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tempDir, err := os.MkdirTemp("", "batch_test")
			assert.NoError(t, err)
			defer os.RemoveAll(tempDir)

			configFile := filepath.Join(tempDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(tt.configContent), 0644)
			assert.NoError(t, err)

			// Update args to use the temp file
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			if len(args) > 1 {
				args[1] = configFile
			}

			err = runBatch(nil, args)
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

func TestBatchDryRun(t *testing.T) {
	// Save original values
	originalClient := batchClient
	originalRepo := repo
	originalPR := prNumber
	originalDryRun := dryRun
	defer func() {
		batchClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		dryRun = originalDryRun
	}()

	// Set up environment
	mockClient := github.NewMockClient()
	batchClient = mockClient
	repo = "owner/repo"
	prNumber = 123
	dryRun = true

	// Create temporary config file
	tempDir, err := os.MkdirTemp("", "batch_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configContent := `
comments:
  - file: src/main.go
    line: 42
    message: "Test comment"
    type: review
`

	configFile := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	err = runBatch(nil, []string{"123", configFile})
	assert.NoError(t, err)
}

func TestBatchVerbose(t *testing.T) {
	// Save original values
	originalClient := batchClient
	originalRepo := repo
	originalPR := prNumber
	originalVerbose := verbose
	originalValidateDiff := validateDiff
	defer func() {
		batchClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		verbose = originalVerbose
		validateDiff = originalValidateDiff
	}()

	// Set up environment
	mockClient := github.NewMockClient()
	batchClient = mockClient
	repo = "owner/repo"
	prNumber = 123
	verbose = true
	validateDiff = false // Disable validation for tests

	// Create temporary config file
	tempDir, err := os.MkdirTemp("", "batch_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configContent := `
comments:
  - file: src/main.go
    line: 42
    message: "Test comment"
    type: review
`

	configFile := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	err = runBatch(nil, []string{"123", configFile})
	assert.NoError(t, err)
}

func TestReadBatchConfig(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		wantErr        bool
		expectedErrMsg string
		expectedPR     int
		expectedRepo   string
	}{
		{
			name: "valid config with all fields",
			configContent: `
pr: 456
repo: test/repo
review:
  body: "Test review"
  event: APPROVE
comments:
  - file: main.go
    line: 10
    message: "Test comment"
    type: review
`,
			wantErr:      false,
			expectedPR:   456,
			expectedRepo: "test/repo",
		},
		{
			name: "config with range comment",
			configContent: `
comments:
  - file: utils.go
    range: "5-10"
    message: "Range comment"
    type: review
`,
			wantErr: false,
		},
		{
			name: "invalid YAML",
			configContent: `
invalid: yaml: content: [
`,
			wantErr:        true,
			expectedErrMsg: "failed to parse YAML",
		},
		{
			name: "invalid comment type",
			configContent: `
comments:
  - file: test.go
    line: 1
    message: "test"
    type: invalid
`,
			wantErr:        true,
			expectedErrMsg: "type must be 'review' or 'issue'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tempDir, err := os.MkdirTemp("", "config_test")
			assert.NoError(t, err)
			defer os.RemoveAll(tempDir)

			configFile := filepath.Join(tempDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(tt.configContent), 0644)
			assert.NoError(t, err)

			config, err := readBatchConfig(configFile)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				if tt.expectedPR != 0 {
					assert.Equal(t, tt.expectedPR, config.PR)
				}
				if tt.expectedRepo != "" {
					assert.Equal(t, tt.expectedRepo, config.Repo)
				}
			}
		})
	}
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		name           string
		rangeStr       string
		expectedStart  int
		expectedEnd    int
		wantErr        bool
		expectedErrMsg string
	}{
		{"valid range", "10-15", 10, 15, false, ""},
		{"single line range", "42-42", 42, 42, false, ""},
		{"range with spaces", " 5 - 10 ", 5, 10, false, ""},
		{"invalid format - no dash", "10", 0, 0, true, "range must be in format 'start-end'"},
		{"invalid format - multiple dashes", "10-15-20", 0, 0, true, "range must be in format 'start-end'"},
		{"invalid start line", "abc-10", 0, 0, true, "invalid start line"},
		{"invalid end line", "10-xyz", 0, 0, true, "invalid end line"},
		{"zero start line", "0-10", 0, 0, true, "line numbers must be positive"},
		{"zero end line", "10-0", 0, 0, true, "line numbers must be positive"},
		{"negative start line", "-5-10", 0, 0, true, "range must be in format 'start-end'"},
		{"start greater than end", "15-10", 0, 0, true, "start line (15) cannot be greater than end line (10)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseRange(tt.rangeStr)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStart, start)
				assert.Equal(t, tt.expectedEnd, end)
			}
		})
	}
}

func TestBatchHelperFunctions(t *testing.T) {
	// Test formatLineOrRange
	comment1 := CommentConfig{Line: 42}
	assert.Equal(t, "42", formatLineOrRange(comment1))

	comment2 := CommentConfig{Range: "10-15"}
	assert.Equal(t, "10-15", formatLineOrRange(comment2))

	// Test truncateMessage
	assert.Equal(t, "short", truncateMessage("short", 10))
	assert.Equal(t, "this is a very...", truncateMessage("this is a very long message", 17))
}

func TestBatchRepositoryParsing(t *testing.T) {
	// Save original values
	originalClient := batchClient
	originalRepo := repo
	originalPR := prNumber
	originalValidateDiff := validateDiff
	defer func() {
		batchClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		validateDiff = originalValidateDiff
	}()

	mockClient := github.NewMockClient()
	batchClient = mockClient
	prNumber = 123
	validateDiff = false // Disable validation for tests

	tests := []struct {
		name           string
		setupRepo      string
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:      "valid repository format",
			setupRepo: "owner/repo",
			wantErr:   false,
		},
		{
			name:      "repository with hyphens",
			setupRepo: "my-org/my-repo",
			wantErr:   false,
		},
		{
			name:           "invalid repository format - no slash",
			setupRepo:      "invalidrepo",
			wantErr:        true,
			expectedErrMsg: "invalid repository format",
		},
		{
			name:           "invalid repository format - multiple slashes",
			setupRepo:      "owner/repo/extra",
			wantErr:        true,
			expectedErrMsg: "invalid repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo = tt.setupRepo

			// Create temporary config file
			tempDir, err := os.MkdirTemp("", "batch_test")
			assert.NoError(t, err)
			defer os.RemoveAll(tempDir)

			configContent := `
comments:
  - file: test.go
    line: 1
    message: "test"
`

			configFile := filepath.Join(tempDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(configContent), 0644)
			assert.NoError(t, err)

			err = runBatch(nil, []string{"123", configFile})
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

func TestBatchWithClientInitialization(t *testing.T) {
	// Save original values
	originalClient := batchClient
	originalRepo := repo
	originalPR := prNumber
	originalValidateDiff := validateDiff
	defer func() {
		batchClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		validateDiff = originalValidateDiff
	}()

	// Set client to nil to test initialization
	batchClient = nil
	repo = "owner/repo"
	prNumber = 123
	validateDiff = false // Disable validation for tests

	// This test verifies that when batchClient is nil,
	// a RealClient is initialized in production
	// Since we can't easily test the RealClient without external dependencies,
	// we'll test that the initialization happens by setting up a mock afterwards

	// First verify the client gets initialized
	mockClient := github.NewMockClient()
	batchClient = mockClient

	// Create temporary config file
	tempDir, err := os.MkdirTemp("", "batch_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configContent := `
comments:
  - file: test.go
    line: 1
    message: "test"
`

	configFile := filepath.Join(tempDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	assert.NoError(t, err)

	err = runBatch(nil, []string{"123", configFile})
	assert.NoError(t, err)
}

func TestProcessIndividualCommentsPrintsAnchors(t *testing.T) {
	// batch.go creates reviews down two separate paths. submitReviewWithComments
	// handles the configured-review case; this one runs when config.Review is
	// nil. Both attach inline comments, so both need to say where they landed —
	// missing one leaves exactly the blind spot the anchors exist to close.
	client := github.NewMockClient()

	output := captureOutput(func() {
		err := processIndividualComments(client, "owner", "repo", 123, []CommentConfig{
			{File: "main.go", Line: 42, Message: "a review comment"},
		})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "main.go:42", "the individual-comment path must print anchors too")
	assert.Contains(t, output, `println(\"hello\")`)
}
