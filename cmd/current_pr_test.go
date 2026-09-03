package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGhExec replaces the gh CLI seam for the duration of a test and records
// the args every call was made with, so tests can assert on the subprocess
// invocation without shelling out.
func stubGhExec(t *testing.T, stdout string, err error) *[][]string {
	t.Helper()

	var calls [][]string
	original := ghExec
	ghExec = func(args ...string) (bytes.Buffer, bytes.Buffer, error) {
		calls = append(calls, args)
		var out, errOut bytes.Buffer
		out.WriteString(stdout)
		return out, errOut, err
	}
	t.Cleanup(func() { ghExec = original })

	return &calls
}

func TestGetCurrentPRForRepoPassesRepoThrough(t *testing.T) {
	// Regression: getCurrentPR() never passed --repo, so it resolved against
	// the current working directory regardless of the flag. That paired a PR
	// number from one repository with the name of another and could land
	// writes on a real-but-unrelated PR.
	calls := stubGhExec(t, "456\n", nil)

	pr, err := getCurrentPRForRepo("owner/target-repo")

	require.NoError(t, err)
	assert.Equal(t, 456, pr)
	require.Len(t, *calls, 1)
	assert.Contains(t, (*calls)[0], "--repo", "branch detection must scope itself to the resolved repository")
	assert.Contains(t, (*calls)[0], "owner/target-repo")
}

func TestGetCurrentPRForRepoOmitsRepoWhenUnknown(t *testing.T) {
	calls := stubGhExec(t, "7\n", nil)

	pr, err := getCurrentPRForRepo("")

	require.NoError(t, err)
	assert.Equal(t, 7, pr)
	require.Len(t, *calls, 1)
	assert.NotContains(t, (*calls)[0], "--repo", "no repository to scope to means no flag")
}

func TestGetCurrentPRForRepoIgnoresPRNumberGlobal(t *testing.T) {
	// Flag precedence belongs to getPRContext. This function reading the
	// global made it look like it owned that decision, which is how a caller
	// ends up assuming it is safe to call directly.
	originalPR := prNumber
	prNumber = 999
	t.Cleanup(func() { prNumber = originalPR })

	calls := stubGhExec(t, "12\n", nil)

	pr, err := getCurrentPRForRepo("owner/repo")

	require.NoError(t, err)
	assert.Equal(t, 12, pr, "should report what gh said, not the --pr global")
	assert.Len(t, *calls, 1)
}

func TestGetCurrentPRForRepoErrors(t *testing.T) {
	t.Run("names the repository it asked about", func(t *testing.T) {
		stubGhExec(t, "", fmt.Errorf("exit status 1"))

		_, err := getCurrentPRForRepo("owner/repo")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "owner/repo")
		assert.Contains(t, err.Error(), "try specifying --pr")
	})

	t.Run("rejects unparseable output", func(t *testing.T) {
		stubGhExec(t, "not-a-number\n", nil)

		_, err := getCurrentPRForRepo("owner/repo")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PR number")
	})
}
