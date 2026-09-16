# Testing Guide

This document explains how to run, write, and maintain tests for the `gh-comment` GitHub CLI extension.

## 🏃‍♂️ Running Tests

### Unit Tests
```bash
# Run all unit tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...

# Run specific test
go test -run TestListCommand ./cmd

# Run tests in verbose mode
go test -v ./...
```

### Integration Tests
```bash
# Run integration tests (requires testscript)
go test -tags=integration ./...

# Run specific integration test
go test -run TestIntegration ./...
```

### Benchmark Tests
```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkListComments ./cmd

# Run benchmarks with memory profiling
go test -bench=. -benchmem ./...

# Use the benchmark script for advanced features
./scripts/benchmark.sh                    # Run benchmarks
./scripts/benchmark.sh compare main       # Compare with main branch
./scripts/benchmark.sh profile            # Generate CPU/memory profiles
```

#### Performance Regression Testing

The project includes automated performance regression testing:

1. **PR Benchmarks**: Every pull request automatically runs benchmarks comparing the PR branch against the base branch. Results are posted as a comment on the PR.

2. **Local Testing**: Use `./scripts/benchmark.sh compare <branch>` to compare performance locally before submitting a PR.

3. **Regression Alerts**: The CI will warn if performance degrades by more than 20%.

### Linting
```bash
# Run golangci-lint
golangci-lint run

# Run with auto-fix
golangci-lint run --fix
```

## 🧪 Test Structure

### Directory Layout
```
gh-comment/
├── cmd/                    # Command implementations
│   ├── list.go
│   ├── list_test.go       # Unit tests for list command
│   ├── reply.go
│   └── reply_test.go      # Unit tests for reply command
├── internal/
│   ├── github/            # GitHub API abstraction
│   │   └── client.go      # Includes mock client
│   └── testutil/          # Test utilities
│       └── helpers.go     # Common test helpers
├── testdata/
│   ├── golden/            # Golden files for output verification
│   │   ├── list_output.txt
│   │   ├── help_text.txt
│   │   └── error_messages.txt
│   └── scripts/           # testscript integration tests
│       ├── list_basic.txtar
│       ├── reply_issue_comment.txtar
│       └── error_handling.txtar
└── integration_test.go    # Integration test runner
```

### Test Types

1. **Unit Tests** (`*_test.go`)
   - Test individual functions and commands
   - Use dependency injection for mocking
   - Fast execution (<1 second total)

2. **Integration Tests** (`testscript`)
   - Test CLI workflows end-to-end
   - Use `.txtar` files for test scenarios
   - Simulate real user interactions

3. **Golden File Tests**
   - Verify CLI output format
   - Compare actual output with expected golden files
   - Update with `UPDATE_GOLDEN=1 go test`

## ✍️ Writing Tests

### Unit Test Example
```go
func TestListCommand(t *testing.T) {
    tests := []struct {
        name         string
        args         []string
        flags        map[string]string
        mockComments []github.Comment
        wantErr      bool
        wantContains []string
    }{
        {
            name: "list basic comments",
            args: []string{"123"},
            mockComments: []github.Comment{
                {
                    ID:   123456,
                    Body: "Test comment",
                    Type: "issue",
                    User: github.User{Login: "testuser"},
                },
            },
            wantContains: []string{"Test comment", "testuser"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Set up mock client
            mockClient := github.NewMockClient()
            mockClient.IssueComments = tt.mockComments

            // Create command with dependency injection
            cmd := NewListCmdWithDeps(&Dependencies{
                GitHubClient: mockClient,
                Output:       &bytes.Buffer{},
            })

            // Run test...
        })
    }
}
```

### Integration Test Example
```
# testdata/scripts/list_basic.txtar
# Test basic list functionality
exec gh-comment list --pr 123 --repo owner/repo
stdout 'General PR Comments'
stdout 'Review Comments'
! stderr .

-- .github/config.yml --
# Mock GitHub CLI config
```

### Golden File Test Example
```go
func TestListOutput(t *testing.T) {
    // Capture command output
    output := captureListOutput()
    
    // Compare with golden file
    testutil.AssertGoldenMatch(t, "list_output.txt", output)
}
```

## 🔧 Test Utilities

### Mock GitHub Client
```go
// Create mock client
mockClient := github.NewMockClient()

// Set up mock data
mockClient.IssueComments = []github.Comment{...}
mockClient.ReviewComments = []github.Comment{...}

// Simulate errors
mockClient.ListIssueCommentsError = fmt.Errorf("API error")
```

### Dependency Injection
```go
type Dependencies struct {
    GitHubClient github.GitHubAPI
    Output       io.Writer
}

func NewListCmdWithDeps(deps *Dependencies) *cobra.Command {
    // Create command with injected dependencies
}
```

### Output Capture
```go
// Capture stdout/stderr
stdout, stderr := testutil.CaptureOutput(func() {
    // Code that writes to stdout/stderr
})
```

### Golden Files
```go
// Update golden files
UPDATE_GOLDEN=1 go test

// Compare with golden file
testutil.AssertGoldenMatch(t, "expected.txt", actual)
```

## 🚀 CI/CD Integration

### GitHub Actions
Tests run automatically on:
- Push to `main` or `develop`
- Pull requests to `main`
- Multiple OS (Ubuntu, macOS, Windows)
- Multiple Go versions (1.23, 1.24)

### Coverage Requirements
- Minimum 80% coverage for critical paths
- Coverage reports uploaded to Codecov
- CI fails if coverage drops below threshold

### Performance Monitoring
- Benchmark tests run on PRs
- Performance regression detection
- Results stored as artifacts

## 🐛 Debugging Tests

### Common Issues

1. **Test Flakiness**
   ```bash
   # Run test multiple times to check for flakiness
   go test -count=10 -run TestSpecificTest
   ```

2. **Race Conditions**
   ```bash
   # Always run with race detector
   go test -race ./...
   ```

3. **Golden File Mismatches**
   ```bash
   # Update golden files when output format changes
   UPDATE_GOLDEN=1 go test
   ```

4. **Mock Setup Issues**
   ```go
   // Reset global state between tests
   func resetFlags() {
       verbose = false
       dryRun = false
       // ... reset other flags
   }
   ```

### Debugging Commands
```bash
# Run single test with verbose output
go test -v -run TestListCommand ./cmd

# Run test with debugging
go test -run TestListCommand -args -test.v

# Check test coverage for specific package
go test -coverprofile=coverage.out ./cmd
go tool cover -func=coverage.out
```

## 📝 Best Practices

### Test Naming
- Use descriptive test names: `TestListCommand_WithAuthorFilter_FiltersCorrectly`
- Group related tests in table-driven format
- Use `t.Run()` for subtests

### Test Organization
- One test file per source file (`list.go` → `list_test.go`)
- Group tests by functionality
- Keep tests close to the code they test

### Mock Usage
- Use interfaces for external dependencies
- Create focused mocks for specific scenarios
- Avoid over-mocking (test behavior, not implementation)

### Assertions
- Use `testify/assert` for readable assertions
- Use `testify/require` for critical assertions that should stop the test
- Provide helpful error messages

### Test Data
- Use table-driven tests for multiple scenarios
- Keep test data minimal and focused
- Use golden files for complex output verification

## 🔄 Maintenance

### Updating Tests
1. **When adding new features**: Write tests first (TDD)
2. **When fixing bugs**: Add regression tests
3. **When refactoring**: Ensure tests still pass
4. **When changing output**: Update golden files

### Performance
- Keep unit tests fast (<1 second total)
- Use `testing.Short()` for long-running tests
- Profile tests if they become slow

### Dependencies
- Keep test dependencies minimal
- Update testing libraries regularly
- Use standard library when possible

This testing strategy ensures high code quality, prevents regressions, and provides confidence when making changes to the codebase.
