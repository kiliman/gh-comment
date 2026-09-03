package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	// Client for dependency injection (tests can override)
	reviewClient github.GitHubAPI

	// Review-specific flags
	reviewEventFlag    string
	reviewCommentsFlag []string
)

var reviewCmd = &cobra.Command{
	Use:   "review <pr> [body]",
	Short: "Create a code review with line-specific comments",
	Long: heredoc.Doc(`
		Create a code review with multiple line-specific comments attached to code.

		This command creates review comments that appear in the "Files Changed" tab,
		attached to specific lines or ranges. Perfect for comprehensive code reviews
		where you want to comment on multiple code locations and submit a review
		decision (APPROVE/REQUEST_CHANGES/COMMENT) in one operation.

		For general PR discussion comments, use: 'gh comment add'
	`),
	Example: heredoc.Doc(`
		# Security-focused comprehensive review
		$ gh comment review 123 "Security audit complete - critical issues found" \
		  --comment src/api.js:6:"Use crypto.randomBytes(32) instead of Math.random() for token generation" \
		  --comment src/api.js:8:10:"This endpoint lacks rate limiting - vulnerable to DoS attacks" \
		  --comment tests/auth_test.js:2:"Input sanitization missing - SQL injection risk" \
		  --event REQUEST_CHANGES

		# Review with comments only (no body text)
		$ gh comment review 123 \
		  --comment src/main.go:3:4:"Extract this N+1 query to a single batch operation" \
		  --comment src/api.js:4:"Consider Redis clustering for this high-traffic endpoint" \
		  --comment src/main.go:4:"Add performance metrics for this critical path" \
		  --event COMMENT

		# Architecture migration approval
		$ gh comment review 123 "Migration to microservices architecture approved" \
		  --comment service-layer.js:45:"Excellent separation of concerns in the new service layer" \
		  --comment api-gateway.go:123:130:"API gateway implementation follows best practices" \
		  --comment docker-compose.yml:67:"Container orchestration setup looks solid" \
		  --event APPROVE

		# Code quality and maintainability review
		$ gh comment review 123 "Code quality review - refactoring needed" \
		  --comment legacy-handler.js:200:250:"This function is doing too much - extract into separate services" \
		  --comment utils.go:45:"Consider using dependency injection pattern here" \
		  --comment test-helpers.js:89:"Add integration tests for this critical business logic" \
		  --event REQUEST_CHANGES
	`),
	Args: cobra.RangeArgs(1, 2),
	RunE: runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().StringVar(&reviewEventFlag, "event", "COMMENT", "Review event (APPROVE|REQUEST_CHANGES|COMMENT) (default: COMMENT)")
	reviewCmd.Flags().StringArrayVar(&reviewCommentsFlag, "comment", []string{}, "Add comment in format file:line:message or file:start:end:message (default: empty)")
}

func runReview(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if reviewClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		reviewClient = client
	}

	var pr int
	var body string
	var err error

	// Parse arguments
	if len(args) == 2 {
		// PR number and body provided
		pr, err = strconv.Atoi(args[0])
		if err != nil {
			return formatValidationError("PR number", args[0], "must be a valid integer")
		}
		body = args[1]
	} else if len(args) == 1 {
		// Check if it's a PR number or review body
		if prNum, err := strconv.Atoi(args[0]); err == nil {
			// It's a PR number, body is optional
			pr = prNum
			body = "" // Body is optional when PR number is provided
		} else {
			// It's a review body, auto-detect PR
			_, pr, err = getPRContext()
			if err != nil {
				return err
			}
			body = args[0]
		}
	}

	// Validate event type
	validEvents := []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"}
	isValidEvent := false
	for _, validEvent := range validEvents {
		if reviewEventFlag == validEvent {
			isValidEvent = true
			break
		}
	}
	if !isValidEvent {
		return fmt.Errorf("invalid event type: %s (must be APPROVE, REQUEST_CHANGES, or COMMENT)", reviewEventFlag)
	}

	// Validate that we have either a body or comments
	if body == "" && len(reviewCommentsFlag) == 0 {
		return fmt.Errorf("review must have either a body message or comments (use --comment flag to add line-specific comments)")
	}

	// Validate review body length if provided
	if body != "" {
		if err := validateCommentBody(body); err != nil {
			return err
		}
	}

	// Get repository context
	repository, err := getCurrentRepo()
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Validate repository name
	if err := validateRepositoryName(repository); err != nil {
		return err
	}

	// If PR number wasn't parsed from args, try to auto-detect it
	if pr == 0 {
		if prNumber > 0 {
			pr = prNumber
		} else {
			detectedPR, err := getCurrentPRForRepo(repository)
			if err != nil {
				return fmt.Errorf("failed to detect PR number: %w", err)
			}
			pr = detectedPR
		}
	}

	// Parse owner/repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repoName := parts[0], parts[1]

	// Parse and validate review comments first (before dry run)
	var reviewCommentInputs []github.ReviewCommentInput
	for i, commentSpec := range reviewCommentsFlag {
		commentInput, err := parseReviewCommentSpec(commentSpec)
		if err != nil {
			return fmt.Errorf("invalid comment %d (%s): %w", i+1, commentSpec, err)
		}

		// Validate line exists in diff if validation is enabled
		if validateDiff {
			if err := validateCommentLine(reviewClient, owner, repoName, pr, commentInput); err != nil {
				return fmt.Errorf("comment %d validation failed (%s): %w", i+1, commentSpec, err)
			}
		}

		reviewCommentInputs = append(reviewCommentInputs, commentInput)
	}

	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("PR: %d\n", pr)
		fmt.Printf("Review body: %s\n", body)
		fmt.Printf("Review event: %s\n", reviewEventFlag)
		fmt.Printf("Comments: %d\n", len(reviewCommentsFlag))
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would create review on PR #%d:\n", pr)
		fmt.Printf("Body: %s\n", body)
		fmt.Printf("Event: %s\n", reviewEventFlag)
		fmt.Printf("Comments: %d\n", len(reviewCommentsFlag))
		for i, comment := range reviewCommentsFlag {
			fmt.Printf("  %d. %s\n", i+1, comment)
		}
		return nil
	}

	// Create the review
	review := github.ReviewInput{
		Body:     body,
		Event:    reviewEventFlag,
		Comments: reviewCommentInputs,
	}

	err = reviewClient.CreateReview(owner, repoName, pr, review)
	if err != nil {
		return formatActionableError("review creation", err)
	}

	// Display success message
	eventText := ""
	switch reviewEventFlag {
	case "APPROVE":
		eventText = "approved"
	case "REQUEST_CHANGES":
		eventText = "requested changes"
	case "COMMENT":
		eventText = "commented on"
	}

	fmt.Printf("✅ Successfully created review and %s PR #%d", eventText, pr)
	if len(reviewCommentInputs) > 0 {
		fmt.Printf(" with %d %s", len(reviewCommentInputs), pluralize("comment", len(reviewCommentInputs)))
	}
	fmt.Println()

	// Show what each comment landed on, so a misplaced line is visible here
	// rather than on the next trip to the browser.
	printCommentAnchors(reviewClient, owner, repoName, pr, reviewCommentInputs)

	return nil
}

// parseReviewCommentSpec parses a comment specification in the format:
// file:line:message or file:start:end:message
func parseReviewCommentSpec(spec string) (github.ReviewCommentInput, error) {
	// Handle quoted messages properly
	var filePath, lineSpec, message string

	// Check if message is quoted
	if strings.Contains(spec, ":'") && strings.HasSuffix(spec, "'") {
		// Message is quoted with single quotes
		quoteIndex := strings.Index(spec, ":'")
		message = spec[quoteIndex+2 : len(spec)-1] // Remove :' and trailing '
		fileAndLine := spec[:quoteIndex]

		// Split file and line
		lastColonIndex := strings.LastIndex(fileAndLine, ":")
		if lastColonIndex == -1 {
			return github.ReviewCommentInput{}, fmt.Errorf("format must be file:line:message or file:start:end:message")
		}
		filePath = fileAndLine[:lastColonIndex]
		lineSpec = fileAndLine[lastColonIndex+1:]
	} else if strings.Contains(spec, ":\"") && strings.HasSuffix(spec, "\"") {
		// Message is quoted with double quotes
		quoteIndex := strings.Index(spec, ":\"")
		message = spec[quoteIndex+2 : len(spec)-1] // Remove :" and trailing "
		fileAndLine := spec[:quoteIndex]

		// Split file and line
		lastColonIndex := strings.LastIndex(fileAndLine, ":")
		if lastColonIndex == -1 {
			return github.ReviewCommentInput{}, fmt.Errorf("format must be file:line:message or file:start:end:message")
		}
		filePath = fileAndLine[:lastColonIndex]
		lineSpec = fileAndLine[lastColonIndex+1:]
	} else {
		// Message is not quoted, use the old logic
		// Try to parse as range format first
		colonCount := strings.Count(spec, ":")
		if colonCount >= 3 {
			// Try file:start:end:message format
			parts := strings.SplitN(spec, ":", 4)
			if len(parts) == 4 {
				// Check if parts[1] and parts[2] are both numbers
				if _, err1 := strconv.Atoi(parts[1]); err1 == nil {
					if _, err2 := strconv.Atoi(parts[2]); err2 == nil {
						// Valid range format
						filePath = parts[0]
						lineSpec = parts[1] + ":" + parts[2]
						message = parts[3]
					}
				}
			}
		}

		// If not range format, try simple format
		if filePath == "" {
			parts := strings.SplitN(spec, ":", 3)
			if len(parts) < 3 {
				return github.ReviewCommentInput{}, fmt.Errorf("format must be file:line:message or file:start:end:message")
			}
			filePath = parts[0]
			lineSpec = parts[1]
			message = strings.Join(parts[2:], ":")
		}
	}

	if filePath == "" {
		return github.ReviewCommentInput{}, fmt.Errorf("file path cannot be empty")
	}
	if message == "" {
		return github.ReviewCommentInput{}, fmt.Errorf("message cannot be empty")
	}

	// Validate file path
	if err := validateFilePath(filePath); err != nil {
		return github.ReviewCommentInput{}, err
	}

	// Validate comment body length
	if err := validateCommentBody(message); err != nil {
		return github.ReviewCommentInput{}, err
	}

	comment := github.ReviewCommentInput{
		Body: expandSuggestions(message),
		Path: filePath,
		Side: "RIGHT", // Default to RIGHT side (additions/new lines)
	}

	// Parse line specification (single line or range)
	if strings.Contains(lineSpec, "-") || strings.Contains(lineSpec, ":") {
		// Range format: start-end or start:end
		var rangeParts []string
		if strings.Contains(lineSpec, "-") {
			rangeParts = strings.Split(lineSpec, "-")
		} else {
			rangeParts = strings.Split(lineSpec, ":")
		}
		if len(rangeParts) != 2 {
			return github.ReviewCommentInput{}, fmt.Errorf("range format must be start-end or start:end")
		}

		startLine, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
		if err != nil {
			return github.ReviewCommentInput{}, fmt.Errorf("invalid start line: %w", err)
		}

		endLine, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
		if err != nil {
			return github.ReviewCommentInput{}, fmt.Errorf("invalid end line: %w", err)
		}

		if startLine <= 0 || endLine <= 0 {
			return github.ReviewCommentInput{}, fmt.Errorf("line numbers must be positive")
		}

		if startLine > endLine {
			return github.ReviewCommentInput{}, fmt.Errorf("start line (%d) cannot be greater than end line (%d)", startLine, endLine)
		}

		comment.StartLine = startLine
		comment.Line = endLine
	} else {
		// Single line format
		line, err := strconv.Atoi(lineSpec)
		if err != nil {
			return github.ReviewCommentInput{}, fmt.Errorf("invalid line number: %w", err)
		}

		if line <= 0 {
			return github.ReviewCommentInput{}, fmt.Errorf("line number must be positive")
		}

		comment.Line = line
	}

	return comment, nil
}

// validateCommentLine validates that the line(s) specified in a review comment exist in the PR diff
func validateCommentLine(client github.GitHubAPI, owner, repo string, pr int, comment github.ReviewCommentInput) error {
	// Fetch PR diff
	diff, err := client.FetchPRDiff(owner, repo, pr)
	if err != nil {
		// If we can't fetch the diff, skip validation rather than blocking the comment
		if verbose {
			fmt.Printf("Warning: Could not fetch PR diff for validation: %v\n", err)
		}
		return nil
	}

	// Find the requested file in the diff
	var targetFile *github.DiffFile
	for i := range diff.Files {
		if diff.Files[i].Filename == comment.Path {
			targetFile = &diff.Files[i]
			break
		}
	}

	if targetFile == nil {
		// Build helpful error message with available files
		var availableFiles []string
		for _, file := range diff.Files {
			availableFiles = append(availableFiles, file.Filename)
		}

		errorMsg := fmt.Sprintf("file '%s' not found in PR #%d diff", comment.Path, pr)
		if len(availableFiles) > 0 {
			errorMsg += "\n\n💡 Available files in this PR:\n"
			for _, filename := range availableFiles {
				errorMsg += fmt.Sprintf("  • %s\n", filename)
			}
			errorMsg += fmt.Sprintf("\nTip: Use 'gh comment lines %d <file>' to see commentable lines", pr)
		}
		return fmt.Errorf("%s", errorMsg)
	}

	// Validate line numbers exist in the diff
	var linesToCheck []int
	if comment.StartLine > 0 {
		// Range comment: check all lines from StartLine to Line
		for line := comment.StartLine; line <= comment.Line; line++ {
			linesToCheck = append(linesToCheck, line)
		}
	} else {
		// Single line comment
		linesToCheck = []int{comment.Line}
	}

	var invalidLines []int
	for _, line := range linesToCheck {
		if !targetFile.Lines[line] {
			invalidLines = append(invalidLines, line)
		}
	}

	if len(invalidLines) > 0 {
		// Build helpful error message with available lines
		var availableLines []int
		for lineNum := range targetFile.Lines {
			availableLines = append(availableLines, lineNum)
		}

		// Sort available lines for better display
		sort.Ints(availableLines)

		// Group consecutive lines for cleaner display
		ranges := groupConsecutiveLines(availableLines)

		var rangeStrings []string
		for _, r := range ranges {
			if r.start == r.end {
				rangeStrings = append(rangeStrings, fmt.Sprintf("%d", r.start))
			} else {
				rangeStrings = append(rangeStrings, fmt.Sprintf("%d-%d", r.start, r.end))
			}
		}

		errorMsg := fmt.Sprintf("line(s) %v do not exist in diff for file '%s'", invalidLines, comment.Path)
		if len(availableLines) > 0 {
			errorMsg += fmt.Sprintf("\n\n💡 Available lines for comments: %s", strings.Join(rangeStrings, ", "))
			errorMsg += fmt.Sprintf("\n\nTip: Use 'gh comment lines %d %s' to see detailed line information", pr, comment.Path)
		} else {
			errorMsg += fmt.Sprintf("\n\n💡 No commentable lines found in '%s' - file may not have changes in this PR", comment.Path)
		}
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}
