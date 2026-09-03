package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/silouanwright/gh-comment/internal/github"
)

func TestRunReviewReplyWithMockClient(t *testing.T) {
	// Save original client and environment
	originalClient := reviewReplyClient
	originalRepo := repo
	originalPR := prNumber
	defer func() {
		reviewReplyClient = originalClient
		repo = originalRepo
		prNumber = originalPR
	}()

	// Set up mock client and environment
	mockClient := github.NewMockClient()
	reviewReplyClient = mockClient
	repo = "owner/repo"
	prNumber = 123

	tests := []struct {
		name           string
		args           []string
		setupResolve   bool
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:    "reply to review comment with message",
			args:    []string{"123456", "test message"},
			wantErr: false,
		},
		{
			name:         "reply to review comment and resolve",
			args:         []string{"123456", "Great point!"},
			setupResolve: true,
			wantErr:      false,
		},
		{
			name:         "resolve review comment without message",
			args:         []string{"123456"},
			setupResolve: true,
			wantErr:      false,
		},
		{
			name:           "invalid comment ID",
			args:           []string{"invalid-id", "message"},
			wantErr:        true,
			expectedErrMsg: "invalid comment ID",
		},
		{
			name:           "missing message and resolve flag",
			args:           []string{"123456"},
			wantErr:        true,
			expectedErrMsg: "must provide either a message or --resolve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up resolve flag
			resolveConversationReviewReply = tt.setupResolve

			err := runReviewReply(nil, tt.args)

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

// INTEGRATION TEST: Document GitHub API limitation for review comment threading
func TestReviewReplyErrorHandling(t *testing.T) {
	// Save original client
	originalClient := reviewReplyClient
	defer func() { reviewReplyClient = originalClient }()

	// Save original globals
	originalRepo := repo
	originalPR := prNumber
	originalResolveConversation := resolveConversationReviewReply
	defer func() {
		repo = originalRepo
		prNumber = originalPR
		resolveConversationReviewReply = originalResolveConversation
	}()

	repo = "owner/repo"
	prNumber = 123
	resolveConversationReviewReply = false // Reset flag for this test

	tests := []struct {
		name        string
		commentID   string
		message     string
		description string
	}{
		{
			name:        "review comment threading limitation",
			commentID:   "123456",
			message:     "This would fail in real GitHub",
			description: "GitHub API doesn't support direct threading for review comments",
		},
		{
			name:        "review comment should work",
			commentID:   "123456",
			message:     "This should work fine",
			description: "Review comment threading is supported by GitHub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up fresh mock client for each test
			mockClient := github.NewMockClient()
			reviewReplyClient = mockClient

			// This test documents the limitation - real implementation would handle the error
			err := runReviewReply(nil, []string{tt.commentID, tt.message})

			// For now, mock client allows it, but real GitHub would reject review comment threading
			t.Logf("LIMITATION: %s", tt.description)

			// Mock client succeeds, but real API has limitations
			assert.NoError(t, err, "Mock client allows all operations, real GitHub has limitations")
		})
	}
}

func TestReviewReplyValidation(t *testing.T) {
	// Save original client and environment
	originalClient := reviewReplyClient
	originalRepo := repo
	originalPR := prNumber
	originalResolve := resolveConversationReviewReply
	defer func() {
		reviewReplyClient = originalClient
		repo = originalRepo
		prNumber = originalPR
		resolveConversationReviewReply = originalResolve
	}()

	// Set up mock client and environment
	mockClient := github.NewMockClient()
	reviewReplyClient = mockClient
	repo = "owner/repo"
	prNumber = 123

	tests := []struct {
		name           string
		args           []string
		setupResolve   bool
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:           "zero comment ID",
			args:           []string{"0", "message"},
			wantErr:        true,
			expectedErrMsg: "must be a positive integer",
		},
		{
			name:           "negative comment ID",
			args:           []string{"-1", "message"},
			wantErr:        true,
			expectedErrMsg: "must be a positive integer",
		},
		{
			name:         "resolve only without message",
			args:         []string{"123456"},
			setupResolve: true,
			wantErr:      false,
		},
		{
			name:    "message with whitespace only",
			args:    []string{"123456", "   \n\t   "},
			wantErr: false, // Whitespace-only message is allowed for review replies
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up resolve flag
			resolveConversationReviewReply = tt.setupResolve

			err := runReviewReply(nil, tt.args)

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

func TestReviewReplyResolveDoesNotWriteBeforeThreadIsKnown(t *testing.T) {
	// The whole point of looking the thread up first: when the lookup fails,
	// nothing has been written, so the error is safe to act on by retrying.
	// Previously the reply went out first and the failure left it committed
	// with no hint that it had.
	originalClient := reviewReplyClient
	originalResolve := resolveConversationReviewReply
	defer func() {
		reviewReplyClient = originalClient
		resolveConversationReviewReply = originalResolve
	}()
	withGlobals(t, "owner/repo", 123)

	mockClient := github.NewMockClient()
	mockClient.FindReviewThreadError = fmt.Errorf("thread not found for comment 3917588327")
	reviewReplyClient = mockClient
	resolveConversationReviewReply = true

	err := runReviewReply(nil, []string{"3917588327", "Verified fixed."})

	require.Error(t, err)
	assert.Nil(t, mockClient.CreatedComment,
		"no reply may be posted when the thread lookup fails — otherwise retrying double-posts")
	assert.Contains(t, err.Error(), "PR #123",
		"the error should name the PR number, which is the input that is usually wrong")
	assert.Contains(t, err.Error(), "safe to retry")
}

func TestReviewReplyReportsPartialWriteWhenResolveFails(t *testing.T) {
	// The reply is already committed by the time resolve runs. If resolve then
	// fails for any reason, say so and hand over the finishing command rather
	// than letting the user re-run this one.
	originalClient := reviewReplyClient
	originalResolve := resolveConversationReviewReply
	defer func() {
		reviewReplyClient = originalClient
		resolveConversationReviewReply = originalResolve
	}()
	withGlobals(t, "owner/repo", 123)

	mockClient := github.NewMockClient()
	mockClient.ResolveThreadError = fmt.Errorf("network unreachable")
	reviewReplyClient = mockClient
	resolveConversationReviewReply = true

	output := captureOutput(func() {
		err := runReviewReply(nil, []string{"3917588327", "Verified fixed."})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve conversation")
	})

	assert.NotNil(t, mockClient.CreatedComment, "the reply did land")
	assert.Contains(t, output, "NOT resolved")
	assert.Contains(t, output, "gh comment resolve 3917588327 --pr 123",
		"should hand over the one-liner that finishes the job")
	assert.Contains(t, output, "would post the reply again",
		"should warn against the retry that would double-post")
}
