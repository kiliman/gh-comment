package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// REGRESSION TESTS FOR API ISOLATION
//
// These tests prevent unit tests from accidentally calling real GitHub APIs,
// which can cause tests to be slow, flaky, or fail due to network issues.
//
// KEY PRINCIPLE: Unit tests should NEVER make real API calls.
// SOLUTION: Always pre-set global variables (prNumber, repo) in test setup.

// TestPreventRealAPICallsRegression - REGRESSION TESTS
// These tests prevent accidentally calling real GitHub APIs in unit tests
// which slows down tests and may fail due to network issues
func TestPreventRealAPICallsRegression(t *testing.T) {
	tests := []struct {
		name        string
		testFunc    func() error
		setupGlobal func()
		description string
	}{
		{
			name: "getPRContext should not call real gh CLI when both globals are set",
			testFunc: func() error {
				// This should NOT call the real gh CLI: repo and prNumber both
				// short-circuit before any subprocess would run.
				gotRepo, pr, err := getPRContext()
				assert.Equal(t, "test/repo", gotRepo)
				assert.Equal(t, 123, pr)
				return err
			},
			setupGlobal: func() {
				repo = "test/repo"
				prNumber = 123 // Pre-set to avoid real API call
			},
			description: "Prevents real gh CLI calls in getPRContext when globals are pre-set",
		},
		{
			name: "getCurrentRepo should not call real gh CLI when repo is set",
			testFunc: func() error {
				// This should NOT call the real gh CLI because repo is already set
				result, err := getCurrentRepo()
				assert.Equal(t, "test/repo", result)
				return err
			},
			setupGlobal: func() {
				repo = "test/repo" // Pre-set to avoid real API call
			},
			description: "Prevents real gh CLI calls in getCurrentRepo when repo is already set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state
			originalPR := prNumber
			originalRepo := repo
			defer func() {
				prNumber = originalPR
				repo = originalRepo
			}()

			// Set up test state to avoid real API calls
			tt.setupGlobal()

			// Run the test function
			err := tt.testFunc()

			// These functions should succeed when globals are pre-set
			assert.NoError(t, err, tt.description)
		})
	}
}

// TestUnitTestsAvoidRealAPICalls verifies that unit tests have proper setup to avoid real API calls
func TestUnitTestsAvoidRealAPICalls(t *testing.T) {
	// This test documents the correct pattern for unit tests:
	// ALWAYS pre-set global variables to avoid accidental real gh CLI calls

	// Save original state
	originalPR := prNumber
	originalRepo := repo
	defer func() {
		prNumber = originalPR
		repo = originalRepo
	}()

	t.Run("functions call real gh CLI when globals unset - by design", func(t *testing.T) {
		// This test documents that getCurrentPR and getCurrentRepo WILL call real gh CLI
		// when globals are unset - this is intentional behavior for production use

		prNumber = 0
		repo = ""

		// Note: We DON'T actually call these functions here because they would
		// trigger real gh CLI calls. This test serves as documentation.

		t.Log("IMPORTANT: getCurrentPRForRepo() and getCurrentRepo() call real gh CLI when globals are unset")
		t.Log("UNIT TEST PATTERN: Always pre-set prNumber and repo variables in test setup")
		t.Log("Example: prNumber = 123; repo = \"owner/repo\" before calling functions")
	})

	t.Run("correct unit test pattern avoids real API calls", func(t *testing.T) {
		// This demonstrates the CORRECT pattern for unit tests
		prNumber = 123
		repo = "test/repo"

		// Now these functions will NOT call real gh CLI
		repoResult, pr, err := getPRContext()
		assert.NoError(t, err)
		assert.Equal(t, 123, pr)
		assert.Equal(t, "test/repo", repoResult)

		repoResult, err = getCurrentRepo()
		assert.NoError(t, err)
		assert.Equal(t, "test/repo", repoResult)

		t.Log("✅ This is the CORRECT pattern: pre-set globals to avoid real API calls")
	})
}

// TestGlobalVariableInitialization ensures globals are properly managed in tests
func TestGlobalVariableInitialization(t *testing.T) {
	// This test documents the expected behavior of global variables
	// and ensures they can be safely mocked in other tests

	// Save original state
	originalPR := prNumber
	originalRepo := repo
	defer func() {
		prNumber = originalPR
		repo = originalRepo
	}()

	// Test that we can set and reset globals safely
	prNumber = 456
	repo = "mock/test"

	assert.Equal(t, 456, prNumber, "prNumber should be settable for testing")
	assert.Equal(t, "mock/test", repo, "repo should be settable for testing")

	// Reset to defaults
	prNumber = 0
	repo = ""

	assert.Equal(t, 0, prNumber, "prNumber should be resettable")
	assert.Equal(t, "", repo, "repo should be resettable")
}
