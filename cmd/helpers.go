package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Constants for API limits and defaults
const (
	MaxGraphQLResults = 100
	MaxCommentLength  = 65536 // GitHub's actual limit for comment body
	MaxFilePathLength = 4096  // Reasonable file path limit
	MaxAuthorLength   = 39    // GitHub username max length
	MaxRepoNameLength = 100   // GitHub repository name max length
	MaxBranchLength   = 255   // Git branch name max length
	DefaultPageSize   = 30

	// Security validation constants
	MaxCommentThreadDepth = 10 // Maximum nested comment thread depth
	MaxRepositoryDepth    = 5  // Maximum repository path depth
	MaxValidationErrors   = 50 // Maximum validation errors to report

	// Display constants
	MaxDisplayBodyLength   = 200 // Max length for comment body display
	TruncationSuffix       = "..."
	TruncationReserve      = 3    // Length of "..."
	SeparatorLength        = 50   // Length of separator lines
	MessageTruncateLength  = 50   // Length for message truncation in batch dry-run
	CommitSHADisplayLength = 8    // Number of characters to show from commit SHA
	DefaultBufferSize      = 4096 // Default buffer size for I/O operations
)

// Security validation patterns
var (
	// HTML/Script tag detection patterns
	htmlTagPattern       = regexp.MustCompile(`(?i)<\s*\/?\s*(script|iframe|object|embed|form|input|meta|link)\b[^>]*>`)
	scriptPattern        = regexp.MustCompile(`(?i)javascript\s*:|<\s*script\b`)
	dangerousAttrPattern = regexp.MustCompile(`(?i)\b(on\w+|javascript|vbscript|mocha|livescript)\s*=`)

	// Markdown code, stripped before the scan above runs. Go's RE2 has no
	// backreferences, so the fence alternatives are spelled out rather than matched
	// to their opener.
	fencedCodePattern = regexp.MustCompile("(?ms)^[ \\t]*(?:`{3,}|~{3,})[^\\n]*\\n.*?^[ \\t]*(?:`{3,}|~{3,})[ \\t]*$")
	inlineCodePattern = regexp.MustCompile("`+[^`\\n]*`+")

	// Repository validation patterns
	validRepoPattern  = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
	validOwnerPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)

// getPRContext gets the repository and PR number, handling both flag and auto-detection
func getPRContext() (repo string, pr int, err error) {
	repo, err = getCurrentRepo()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get repository: %w", err)
	}

	if prNumber > 0 {
		pr = prNumber
	} else {
		pr, err = getCurrentPR()
		if err != nil {
			return "", 0, fmt.Errorf("failed to detect PR number: %w (try specifying --pr)", err)
		}
	}

	return repo, pr, nil
}

// formatAPIError creates consistent error messages for API failures
func formatAPIError(operation, endpoint string, err error) error {
	return fmt.Errorf("GitHub API error during %s: %w", operation, err)
}

// formatActionableError creates user-friendly error messages with actionable suggestions
func formatActionableError(operation string, err error) error {
	errStr := err.Error()

	// Handle common GitHub API error patterns
	switch {
	case containsAny(errStr, []string{"422", "Unprocessable Entity", "validation failed"}):
		return fmt.Errorf("validation error during %s: %w\n\n💡 Suggestions:\n  • Check if the line number exists in the PR diff\n  • Use 'gh comment lines <pr> <file>' to see commentable lines\n  • Verify the file path is correct in the PR\n  • For line-specific comments, ensure the line was modified in this PR", operation, err)

	case containsAny(errStr, []string{"404", "Not Found"}):
		return fmt.Errorf("resource not found during %s: %w\n\n💡 Suggestions:\n  • Verify the PR number exists and is accessible\n  • Check if the comment ID is valid\n  • Ensure you have permission to access this repository", operation, err)

	case containsAny(errStr, []string{"403", "Forbidden", "Resource not accessible by integration"}):
		return fmt.Errorf("permission denied during %s: %w\n\n💡 Suggestions:\n  • Check if you have write access to the repository\n  • Verify your GitHub authentication with 'gh auth status'\n  • You cannot approve your own PR or comment on private repos without access\n  • For organization repos, check if your token has the required scopes", operation, err)

	case containsAny(errStr, []string{"401", "Unauthorized", "Bad credentials"}):
		return fmt.Errorf("authentication failed during %s: %w\n\n💡 Suggestions:\n  • Run 'gh auth login' to authenticate with GitHub\n  • Check if your token has expired with 'gh auth status'\n  • Verify you're authenticated with the correct GitHub account\n  • For personal access tokens, ensure they haven't been revoked", operation, err)

	case containsAny(errStr, []string{"rate limit", "rate_limit", "too many requests", "API rate limit exceeded"}):
		return fmt.Errorf("rate limit exceeded during %s: %w\n\n💡 Suggestions:\n  • Wait until your rate limit resets (check headers or try in 1 hour)\n  • Use authenticated requests (ensure 'gh auth status' shows logged in)\n  • Consider reducing the frequency of API calls\n  • Check current rate limit: gh api rate_limit", operation, err)

	case containsAny(errStr, []string{"500", "502", "503", "504", "Internal Server Error", "Bad Gateway", "Service Unavailable", "Gateway Timeout"}):
		return fmt.Errorf("GitHub server error during %s: %w\n\n💡 Suggestions:\n  • This is a temporary GitHub server issue\n  • Try again in a few minutes with exponential backoff\n  • Check GitHub's status page at https://status.github.com\n  • For 504 errors, try smaller batch sizes if applicable", operation, err)

	case containsAny(errStr, []string{"timeout", "context deadline exceeded", "request timeout"}):
		return fmt.Errorf("network timeout during %s: %w\n\n💡 Suggestions:\n  • Check your internet connection stability\n  • Try again with a more stable network connection\n  • For large operations, consider breaking them into smaller chunks\n  • Increase timeout settings if configurable", operation, err)

	case containsAny(errStr, []string{"connection refused", "connection reset", "network unreachable"}):
		return fmt.Errorf("network connection error during %s: %w\n\n💡 Suggestions:\n  • Check your internet connection\n  • Verify GitHub is accessible from your network\n  • Check if you're behind a corporate firewall\n  • Try using a different network or VPN", operation, err)

	case containsAny(errStr, []string{"GraphQL", "Field", "doesn't exist", "Unknown field", "syntax error"}):
		return fmt.Errorf("GraphQL API error during %s: %w\n\n💡 Suggestions:\n  • This is likely a bug in the tool's GraphQL queries\n  • Try using the REST API fallback if available\n  • Update to the latest version of the tool\n  • Report this issue to the tool maintainers", operation, err)

	case containsAny(errStr, []string{"secondary rate limit", "abuse detection"}):
		return fmt.Errorf("GitHub abuse detection triggered during %s: %w\n\n💡 Suggestions:\n  • You're making requests too rapidly\n  • Wait at least 1 minute before retrying\n  • Implement delays between requests\n  • Reduce concurrent operations", operation, err)

	case containsAny(errStr, []string{"repository archived", "read-only", "archived"}):
		return fmt.Errorf("repository is archived during %s: %w\n\n💡 Suggestions:\n  • You cannot modify archived repositories\n  • Ask the repository owner to unarchive it\n  • Fork the repository if you need to make changes", operation, err)

	case containsAny(errStr, []string{"token does not have", "insufficient scope", "scope"}):
		return fmt.Errorf("insufficient token permissions during %s: %w\n\n💡 Suggestions:\n  • Your token lacks required scopes for this operation\n  • Re-authenticate with 'gh auth login' with broader scopes\n  • For personal access tokens, check required scopes in GitHub settings\n  • Ensure token has 'repo' scope for private repositories", operation, err)

	case containsAny(errStr, []string{"branch protection", "required status checks", "protected branch"}):
		return fmt.Errorf("branch protection rules violated during %s: %w\n\n💡 Suggestions:\n  • The target branch has protection rules enabled\n  • Required status checks may need to pass first\n  • Ask a repository administrator for required permissions\n  • Check branch protection settings in repository settings", operation, err)

	case containsAny(errStr, []string{"pull request closed", "issue closed", "locked conversation"}):
		return fmt.Errorf("target is closed or locked during %s: %w\n\n💡 Suggestions:\n  • You cannot comment on closed/locked issues or PRs\n  • Ask a maintainer to reopen if necessary\n  • Create a new issue or PR if appropriate", operation, err)

	case containsAny(errStr, []string{"No subschema in oneOf matched", "invalid request", "malformed"}):
		return fmt.Errorf("invalid request format during %s: %w\n\n💡 Suggestions:\n  • Check the command syntax in --help\n  • Verify all required arguments are provided\n  • For line comments, ensure the line exists in the PR diff\n  • Check for special characters that need escaping", operation, err)

	case containsAny(errStr, []string{"pending review", "draft review", "cannot create review", "review in progress", "unsubmitted review"}):
		return fmt.Errorf("pending review blocks new review during %s: %w\n\n💡 GitHub API Limitation:\n  • You have an unsubmitted draft review on this PR\n  • GitHub only allows one review per user at a time\n  • You must submit or abandon your pending review first\n\n🔧 Solution:\n  • Run: gh comment close-pending-review <pr> \"Submitting draft review\" --event COMMENT\n  • This will submit your pending review so you can create new ones\n  • Alternative: Go to GitHub web UI → Review changes → Abandon review", operation, err)

	case containsAny(errStr, []string{"review already submitted", "duplicate"}):
		return fmt.Errorf("duplicate operation during %s: %w\n\n💡 Suggestions:\n  • This review or comment already exists\n  • Use 'gh comment list' to check existing comments\n  • Use edit operations to modify existing content\n  • Dismiss existing reviews before submitting new ones", operation, err)
	}

	// Default case - provide general guidance
	return fmt.Errorf("error during %s: %w\n\n💡 Suggestions:\n  • Check 'gh comment --help' for correct usage\n  • Verify PR number and file paths are correct\n  • Run with --verbose for more details\n  • Check GitHub's API status at https://status.github.com", operation, err)
}

// containsAny checks if a string contains any of the provided substrings (case-insensitive)
func containsAny(str string, substrings []string) bool {
	lowerStr := strings.ToLower(str)
	for _, substr := range substrings {
		if strings.Contains(lowerStr, strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

// formatValidationError creates consistent error messages for validation failures
func formatValidationError(field, value, expected string) error {
	return fmt.Errorf("invalid %s '%s': %s", field, value, expected)
}

// formatSecurityValidationError creates consistent error messages for security validation failures
func formatSecurityValidationError(field, issue, guidance string) error {
	return fmt.Errorf("🔒 security validation failed for %s: %s\n\n💡 Guidance: %s", field, issue, guidance)
}

// formatAccessValidationError creates consistent error messages for access validation failures
func formatAccessValidationError(resource, action, guidance string) error {
	return fmt.Errorf("🚫 access validation failed for %s: cannot %s\n\n💡 Guidance: %s", resource, action, guidance)
}

// formatNotFoundError creates consistent error messages for missing resources
func formatNotFoundError(resource string, identifier interface{}) error {
	return fmt.Errorf("%s not found: %v", resource, identifier)
}

// parsePositiveInt parses a string to a positive integer with consistent validation
func parsePositiveInt(s, fieldName string) (int, error) {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, formatValidationError(fieldName, s, "must be a valid integer")
	}
	if val <= 0 {
		return 0, formatValidationError(fieldName, s, "must be a positive integer")
	}
	return val, nil
}

// validateCommentBody validates comment body length and content for security
func validateCommentBody(body string) error {
	if len(body) > MaxCommentLength {
		return formatValidationError("comment body", fmt.Sprintf("%d chars", len(body)),
			fmt.Sprintf("must be %d characters or less", MaxCommentLength))
	}

	// Check for potentially dangerous HTML/script content
	if err := validateCommentSecurity(body); err != nil {
		return err
	}

	return nil
}

// stripMarkdownCode removes fenced blocks and inline spans so the security scan
// reads prose only. Quoting source is what a review comment is for, and the body
// leaves here as a JSON API payload — GitHub's own renderer sanitizes it, nothing
// executes it on this side.
func stripMarkdownCode(body string) string {
	return inlineCodePattern.ReplaceAllString(fencedCodePattern.ReplaceAllString(body, ""), "")
}

// validateCommentSecurity checks for potentially malicious content in comments
func validateCommentSecurity(body string) error {
	body = stripMarkdownCode(body)

	// Check for dangerous HTML tags
	if htmlTagPattern.MatchString(body) {
		return formatSecurityValidationError("comment body", "dangerous HTML tags detected",
			"HTML tags like <script>, <iframe>, <object>, <embed>, <form>, <input>, <meta>, <link> are not allowed")
	}

	// Check for JavaScript and event handlers
	if scriptPattern.MatchString(body) {
		return formatSecurityValidationError("comment body", "JavaScript content detected",
			"JavaScript code, inline event handlers, and script tags are not allowed")
	}

	// Check for dangerous attributes
	if dangerousAttrPattern.MatchString(body) {
		return formatSecurityValidationError("comment body", "dangerous attributes detected",
			"Event handlers and script attributes like onclick, onload, href='javascript:' are not allowed outside code blocks — wrap quoted source in backticks")
	}

	return nil
}

// validateFilePath validates file path length and format to prevent directory traversal
func validateFilePath(path string) error {
	if len(path) > MaxFilePathLength {
		return fmt.Errorf("file path too long: %d characters (maximum %d allowed)", len(path), MaxFilePathLength)
	}

	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid file path: directory traversal not allowed")
	}

	// Check for absolute paths (should be relative to repo root)
	if strings.HasPrefix(path, "/") {
		return fmt.Errorf("invalid file path: absolute paths not allowed, use relative paths from repository root")
	}

	return nil
}

// validateRepositoryName validates GitHub repository name format
func validateRepositoryName(repo string) error {
	if len(repo) > MaxRepoNameLength {
		return fmt.Errorf("repository name too long: %d characters (maximum %d allowed)", len(repo), MaxRepoNameLength)
	}

	// Check for basic owner/repo format
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: must be 'owner/repo'")
	}

	owner, repoName := parts[0], parts[1]
	if len(owner) == 0 || len(repoName) == 0 {
		return fmt.Errorf("invalid repository format: owner and repository name cannot be empty")
	}

	if len(owner) > MaxAuthorLength {
		return fmt.Errorf("repository owner too long: %d characters (maximum %d allowed)", len(owner), MaxAuthorLength)
	}

	return nil
}

// validateRepositoryAccess validates if a repository format allows safe access
func validateRepositoryAccess(repo string) error {
	// First validate the basic format
	if err := validateRepositoryName(repo); err != nil {
		return err
	}

	parts := strings.Split(repo, "/")
	owner, repoName := parts[0], parts[1]

	// Check for potentially dangerous repository patterns
	if strings.Contains(owner, "..") || strings.Contains(repoName, "..") {
		return formatAccessValidationError("repository", "access repository with path traversal",
			"Repository names cannot contain '..' sequences for security reasons")
	}

	// Validate against pattern for additional security
	if !validRepoPattern.MatchString(repo) {
		return formatAccessValidationError("repository", "access repository with invalid characters",
			"Repository must match pattern 'owner/repo' with only alphanumeric, dot, underscore, and hyphen characters")
	}

	// Check for reserved or potentially dangerous names
	dangerousNames := []string{".", "..", "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	ownerUpper := strings.ToUpper(owner)
	repoUpper := strings.ToUpper(repoName)

	for _, dangerous := range dangerousNames {
		if ownerUpper == dangerous || repoUpper == dangerous {
			return formatAccessValidationError("repository", "access repository with reserved name",
				"Repository owner or name cannot use reserved system names")
		}
	}

	return nil
}

// validateCommentThreadDepth validates comment thread nesting depth for performance and UX
func validateCommentThreadDepth(depth int) error {
	if depth < 0 {
		return formatValidationError("thread depth", fmt.Sprintf("%d", depth), "must be non-negative")
	}

	if depth > MaxCommentThreadDepth {
		return formatAccessValidationError("comment thread", fmt.Sprintf("nest deeper than %d levels", MaxCommentThreadDepth),
			fmt.Sprintf("Deep comment nesting can cause performance issues and poor user experience. Maximum allowed depth is %d levels", MaxCommentThreadDepth))
	}

	return nil
}

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Field   string
	Valid   bool
	Error   error
	Message string
}

// validateMultipleFields validates multiple fields and returns comprehensive results
func validateMultipleFields(validations []func() ValidationResult) []ValidationResult {
	var results []ValidationResult
	for _, validate := range validations {
		result := validate()
		results = append(results, result)

		// Stop early if we hit the max validation errors
		if len(results) >= MaxValidationErrors {
			break
		}
	}
	return results
}

// createFieldValidator creates a validation function for a specific field
func createFieldValidator(field string, value interface{}, validator func() error) func() ValidationResult {
	return func() ValidationResult {
		err := validator()
		return ValidationResult{
			Field:   field,
			Valid:   err == nil,
			Error:   err,
			Message: fmt.Sprintf("Validation for %s: %v", field, value),
		}
	}
}

// lineRange represents a range of consecutive line numbers for display
type lineRange struct {
	start, end int
}

// groupConsecutiveLines groups consecutive line numbers into ranges for better display
func groupConsecutiveLines(lines []int) []lineRange {
	if len(lines) == 0 {
		return nil
	}

	var ranges []lineRange
	start := lines[0]
	end := lines[0]

	for i := 1; i < len(lines); i++ {
		if lines[i] == end+1 {
			// Consecutive line, extend the range
			end = lines[i]
		} else {
			// Gap found, close current range and start new one
			ranges = append(ranges, lineRange{start: start, end: end})
			start = lines[i]
			end = lines[i]
		}
	}

	// Add the final range
	ranges = append(ranges, lineRange{start: start, end: end})

	return ranges
}
