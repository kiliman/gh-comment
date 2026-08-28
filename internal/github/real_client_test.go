package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
)

func TestNewRealClient(t *testing.T) {
	client, err := NewRealClient()

	// This might fail in test environment if GitHub CLI isn't set up
	// but we still want to test the function is callable
	if err != nil {
		// Expected in test environment - just verify error is reasonable
		assert.Contains(t, err.Error(), "failed to create")
		return
	}

	assert.NotNil(t, client)
	assert.NotNil(t, client.restClient)
	assert.NotNil(t, client.graphqlClient)
}

func TestRealClientValidation(t *testing.T) {
	client := &RealClient{} // Don't need actual API clients for validation tests

	t.Run("ListIssueComments validation", func(t *testing.T) {
		tests := []struct {
			name     string
			owner    string
			repo     string
			prNumber int
			wantErr  string
		}{
			{
				name:     "empty owner",
				owner:    "",
				repo:     "repo",
				prNumber: 123,
				wantErr:  "repository owner cannot be empty",
			},
			{
				name:     "empty repo",
				owner:    "owner",
				repo:     "",
				prNumber: 123,
				wantErr:  "repository name cannot be empty",
			},
			{
				name:     "invalid PR number",
				owner:    "owner",
				repo:     "repo",
				prNumber: -1,
				wantErr:  "invalid PR number -1: must be positive",
			},
			{
				name:     "owner with slash",
				owner:    "own/er",
				repo:     "repo",
				prNumber: 123,
				wantErr:  "invalid repository format",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := client.ListIssueComments(tt.owner, tt.repo, tt.prNumber)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("ListReviewComments validation", func(t *testing.T) {
		_, err := client.ListReviewComments("", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.ListReviewComments("owner", "", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository name cannot be empty")

		_, err = client.ListReviewComments("owner", "repo", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")
	})

	t.Run("CreateIssueComment validation", func(t *testing.T) {
		_, err := client.CreateIssueComment("", "repo", 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.CreateIssueComment("owner", "", 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository name cannot be empty")

		_, err = client.CreateIssueComment("owner", "repo", -1, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number -1: must be positive")

		_, err = client.CreateIssueComment("owner", "repo", 123, "  ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "comment body cannot be empty")
	})

	t.Run("CreateReviewCommentReply validation", func(t *testing.T) {
		_, err := client.CreateReviewCommentReply("", "repo", 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.CreateReviewCommentReply("owner", "repo", 0, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID 0: must be positive")

		_, err = client.CreateReviewCommentReply("owner", "repo", 123, "  ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reply body cannot be empty")
	})

	t.Run("AddReaction validation", func(t *testing.T) {
		err := client.AddReaction("", "repo", 123, 123, "+1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.AddReaction("owner", "repo", 0, 123, "+1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID 0: must be positive")

		err = client.AddReaction("owner", "repo", 123, 123, "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reaction 'invalid'")
	})

	t.Run("RemoveReaction validation", func(t *testing.T) {
		err := client.RemoveReaction("", "repo", 123, 123, "+1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.RemoveReaction("owner", "repo", 0, 123, "+1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID 0: must be positive")

		err = client.RemoveReaction("owner", "repo", 123, 123, "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reaction 'invalid'")
	})

	t.Run("EditComment validation", func(t *testing.T) {
		err := client.EditComment("", "repo", 123, 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.EditComment("owner", "repo", 0, 123, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID 0: must be positive")

		err = client.EditComment("owner", "repo", 123, 123, "  ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "comment body cannot be empty")
	})

	t.Run("AddReviewComment validation", func(t *testing.T) {
		comment := ReviewCommentInput{
			Body: "test",
			Path: "test.go",
			Line: 42,
		}

		err := client.AddReviewComment("", "repo", 123, comment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.AddReviewComment("owner", "repo", 0, comment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")

		emptyBodyComment := ReviewCommentInput{
			Body: "  ",
			Path: "test.go",
			Line: 42,
		}
		err = client.AddReviewComment("owner", "repo", 123, emptyBodyComment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "review comment body cannot be empty")

		emptyPathComment := ReviewCommentInput{
			Body: "test",
			Path: "",
			Line: 42,
		}
		err = client.AddReviewComment("owner", "repo", 123, emptyPathComment)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "review comment path cannot be empty")
	})

	t.Run("FetchPRDiff validation", func(t *testing.T) {
		_, err := client.FetchPRDiff("", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.FetchPRDiff("owner", "repo", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")
	})

	t.Run("FindReviewThreadForComment validation", func(t *testing.T) {
		_, err := client.FindReviewThreadForComment("", "repo", 123, 456)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.FindReviewThreadForComment("owner", "repo", 0, 456)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")

		_, err = client.FindReviewThreadForComment("owner", "repo", 123, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid comment ID 0: must be positive")
	})

	t.Run("ResolveReviewThread validation", func(t *testing.T) {
		err := client.ResolveReviewThread("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "thread ID cannot be empty")

		err = client.ResolveReviewThread("  ")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "thread ID cannot be empty")
	})

	t.Run("CreateReview validation", func(t *testing.T) {
		review := ReviewInput{
			Body:  "test review",
			Event: "APPROVE",
		}

		err := client.CreateReview("", "repo", 123, review)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.CreateReview("owner", "repo", 0, review)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")

		invalidReview := ReviewInput{
			Body:  "test review",
			Event: "INVALID_EVENT",
		}
		err = client.CreateReview("owner", "repo", 123, invalidReview)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid review event 'INVALID_EVENT'")
	})

	t.Run("GetPRDetails validation", func(t *testing.T) {
		_, err := client.GetPRDetails("", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.GetPRDetails("owner", "repo", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")
	})

	t.Run("FindPendingReview validation", func(t *testing.T) {
		_, err := client.FindPendingReview("", "repo", 123)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		_, err = client.FindPendingReview("owner", "repo", 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")
	})

	t.Run("SubmitReview validation", func(t *testing.T) {
		err := client.SubmitReview("", "repo", 123, 456, "test", "APPROVE")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository owner cannot be empty")

		err = client.SubmitReview("owner", "repo", 0, 456, "test", "APPROVE")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number 0: must be positive")

		err = client.SubmitReview("owner", "repo", 123, 0, "test", "APPROVE")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid review ID 0: must be positive")

		err = client.SubmitReview("owner", "repo", 123, 456, "test", "INVALID")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid review event 'INVALID'")
	})
}

func TestWrapAPIErrorInRealClient(t *testing.T) {
	client := &RealClient{}

	tests := []struct {
		name           string
		err            error
		operation      string
		args           []interface{}
		wantContains   []string
		wantTipKeyword string
	}{
		{
			name:           "rate limit error",
			err:            errors.New("rate limit exceeded"),
			operation:      "test operation on %s",
			args:           []interface{}{"repo"},
			wantContains:   []string{"rate limit exceeded", "test operation on repo"},
			wantTipKeyword: "💡 Tips:",
		},
		{
			name:           "403 error",
			err:            errors.New("HTTP 403 Forbidden"),
			operation:      "access resource %d",
			args:           []interface{}{123},
			wantContains:   []string{"rate limit exceeded", "access resource 123"},
			wantTipKeyword: "gh api rate_limit",
		},
		{
			name:           "404 error",
			err:            errors.New("HTTP 404 Not Found"),
			operation:      "find resource %s/%s",
			args:           []interface{}{"owner", "repo"},
			wantContains:   []string{"resource not found", "find resource owner/repo"},
			wantTipKeyword: "Verify the repository exists",
		},
		{
			name:           "401 error",
			err:            errors.New("HTTP 401 Unauthorized"),
			operation:      "authenticate user",
			args:           []interface{}{},
			wantContains:   []string{"authentication failed", "authenticate user"},
			wantTipKeyword: "gh auth status",
		},
		{
			name:           "422 error",
			err:            errors.New("HTTP 422 Unprocessable Entity"),
			operation:      "validate input %s",
			args:           []interface{}{"data"},
			wantContains:   []string{"validation error", "validate input data"},
			wantTipKeyword: "Check that your input parameters",
		},
		{
			name:           "secondary rate limit",
			err:            errors.New("abuse detection triggered"),
			operation:      "create multiple items",
			args:           []interface{}{},
			wantContains:   []string{"secondary rate limit", "create multiple items"},
			wantTipKeyword: "Wait 60 seconds",
		},
		{
			name:           "generic error",
			err:            errors.New("network timeout"),
			operation:      "perform request",
			args:           []interface{}{},
			wantContains:   []string{"GitHub API error", "perform request"},
			wantTipKeyword: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.wrapAPIError(tt.err, tt.operation, tt.args...)

			// Check that original error is wrapped
			assert.True(t, errors.Is(result, tt.err))

			resultStr := result.Error()

			// Check required content
			for _, want := range tt.wantContains {
				assert.Contains(t, resultStr, want)
			}

			// Check for tip keyword if specified
			if tt.wantTipKeyword != "" {
				assert.Contains(t, resultStr, tt.wantTipKeyword)
			}
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	client := &RealClient{}

	// This is currently a no-op function, but we can test it's callable
	client.checkRateLimit()

	// No assertions needed - just verify the function exists and doesn't panic
}

func TestParseDiff(t *testing.T) {
	tests := []struct {
		name        string
		diffContent string
		wantFiles   int
	}{
		{
			name:        "empty diff",
			diffContent: "",
			wantFiles:   0,
		},
		{
			name: "single file diff",
			diffContent: `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,4 @@
 func main() {
+    fmt.Println("hello")
     return
 }`,
			wantFiles: 1,
		},
		{
			name: "multiple file diff",
			diffContent: `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1,3 +1,4 @@
 func test1() {
 }
diff --git a/file2.go b/file2.go
index 7654321..gfedcba 100644
--- a/file2.go
+++ b/file2.go
@@ -1,3 +1,4 @@
 func test2() {
 }`,
			wantFiles: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiff(tt.diffContent)

			assert.NotNil(t, result)
			assert.Len(t, result.Files, tt.wantFiles)

			// Verify structure
			if tt.wantFiles > 0 {
				for _, file := range result.Files {
					assert.NotEmpty(t, file.Filename)
					assert.NotNil(t, file.Lines)
				}
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchPRDiffUsesAPIEndpointWithDiffAcceptHeader(t *testing.T) {
	restTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "token test-token", req.Header.Get("Authorization"))
		assert.Equal(t, "https://api.github.com/repos/owner/repo/pulls/123", req.URL.String())

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"number":123}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	diffTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, "token test-token", req.Header.Get("Authorization"))
		assert.Equal(t, "application/vnd.github.v3.diff", req.Header.Get("Accept"))
		assert.Equal(t, "https://api.github.com/repos/owner/repo/pulls/123", req.URL.String())

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,2 +1,3 @@
 func main() {
+    println("hello")
 }`)),
			Header: http.Header{"Content-Type": []string{"text/plain"}},
		}, nil
	})

	restClient, err := api.NewRESTClient(api.ClientOptions{
		AuthToken: "test-token",
		Host:      "github.com",
		Transport: restTransport,
	})
	assert.NoError(t, err)

	diffClient, err := api.NewRESTClient(api.ClientOptions{
		AuthToken: "test-token",
		Host:      "github.com",
		Headers: map[string]string{
			"Accept": "application/vnd.github.v3.diff",
		},
		Transport: diffTransport,
	})
	assert.NoError(t, err)

	client := &RealClient{restClient: restClient, diffClient: diffClient}
	diff, err := client.FetchPRDiff("owner", "repo", 123)
	assert.NoError(t, err)
	assert.NotNil(t, diff)
	assert.Len(t, diff.Files, 1)
	assert.Equal(t, "test.go", diff.Files[0].Filename)
	assert.True(t, diff.Files[0].Lines[1])
	assert.True(t, diff.Files[0].Lines[2])
	assert.True(t, diff.Files[0].Lines[3])
}
