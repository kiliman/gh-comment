package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRNumberFromParentURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr string
	}{
		{
			name: "review comment parent",
			url:  "https://api.github.com/repos/beehiiv/swarm/pulls/29065",
			want: 29065,
		},
		{
			// An issue comment's issue number is the PR number when the issue
			// is a PR, so the same parse serves both namespaces.
			name: "issue comment parent",
			url:  "https://api.github.com/repos/beehiiv/swarm/issues/29065",
			want: 29065,
		},
		{
			name: "tolerates a trailing slash",
			url:  "https://api.github.com/repos/owner/repo/pulls/422/",
			want: 422,
		},
		{
			name:    "empty",
			url:     "",
			wantErr: "no parent URL",
		},
		{
			name:    "whitespace only",
			url:     "   ",
			wantErr: "no parent URL",
		},
		{
			name:    "non-numeric tail",
			url:     "https://api.github.com/repos/owner/repo/pulls/not-a-number",
			wantErr: "could not parse a PR number",
		},
		{
			name:    "zero is not a PR",
			url:     "https://api.github.com/repos/owner/repo/pulls/0",
			wantErr: "could not parse a PR number",
		},
		{
			name:    "no path separator",
			url:     "29065",
			wantErr: "unexpected parent URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prNumberFromParentURL(tt.url)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPRNumberForCommentValidatesInput(t *testing.T) {
	client := &RealClient{}

	_, err := client.GetPRNumberForComment("", "repo", 1)
	assert.Error(t, err, "owner is required")

	_, err = client.GetPRNumberForComment("owner", "repo", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	_, err = client.GetPRNumberForComment("owner", "repo", -5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}
