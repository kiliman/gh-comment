package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	// Client for dependency injection (tests can override)
	batchClient github.GitHubAPI
)

// BatchConfig represents the structure of a batch comment configuration file
type BatchConfig struct {
	PR       int             `yaml:"pr,omitempty"`
	Repo     string          `yaml:"repo,omitempty"`
	Review   *ReviewConfig   `yaml:"review,omitempty"`
	Comments []CommentConfig `yaml:"comments,omitempty"`
}

// ReviewConfig represents review-level configuration
type ReviewConfig struct {
	Body  string `yaml:"body,omitempty"`
	Event string `yaml:"event,omitempty"` // APPROVE, REQUEST_CHANGES, COMMENT
}

// CommentConfig represents individual comment configuration
type CommentConfig struct {
	File    string `yaml:"file"`
	Line    int    `yaml:"line,omitempty"`
	Range   string `yaml:"range,omitempty"` // e.g., "10-15"
	Message string `yaml:"message"`
	Type    string `yaml:"type,omitempty"` // "review" or "issue", defaults to "review"
}

var batchCmd = &cobra.Command{
	Use:   "batch <pr> <config-file>",
	Short: "Process multiple comments from a YAML configuration file",
	Long: heredoc.Doc(`
		Process multiple comments, reactions, and reviews from a YAML configuration file.

		This is ideal for bulk operations, automated workflows, or complex review
		scenarios. The config file can specify mixed comment types, create reviews
		with multiple comments, and set up entire review workflows.

		YAML Configuration:
		- Review level: Use 'body' field for review summary
		- Individual comments: Use 'message' field for comment text
		- PR can be specified in file or via command line (CLI takes precedence)
	`),
	Example: heredoc.Doc(`
		# Process comments from config
		$ gh comment batch 123 review-config.yaml

		# Validate config without executing
		$ gh comment batch 123 review-config.yaml --dry-run

		# Use verbose output
		$ gh comment batch 123 review-config.yaml --verbose
	`),
	Args: cobra.ExactArgs(2),
	RunE: runBatch,
}

func init() {
	rootCmd.AddCommand(batchCmd)
}

func runBatch(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if batchClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		batchClient = client
	}

	// Validate and prepare batch configuration
	owner, repoName, pr, config, configFile, err := validateBatchConfig(args)
	if err != nil {
		return err
	}

	// Handle verbose output and dry run
	isDryRun := processBatchItems(config, configFile, pr)
	if isDryRun {
		return nil // Dry run completed successfully
	}

	// Execute the batch processing
	return handleBatchResults(batchClient, owner, repoName, pr, config)
}

// validateBatchConfig handles parsing, validation, and setup of batch configuration
func validateBatchConfig(args []string) (owner, repoName string, pr int, config *BatchConfig, configFile string, err error) {
	// Parse PR number
	prArg := args[0]
	pr, err = strconv.Atoi(prArg)
	if err != nil {
		return "", "", 0, nil, "", formatValidationError("PR number", prArg, "must be a valid integer")
	}

	// Read and parse configuration file
	configFile = args[1]
	config, err = readBatchConfig(configFile)
	if err != nil {
		return "", "", 0, nil, "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Override PR and repo from config if specified
	if config.PR != 0 {
		pr = config.PR
	}

	repository := repo
	if config.Repo != "" {
		repository = config.Repo
	}

	// Get repository context if not specified (but keep PR from argument/config)
	if repository == "" {
		repository, err = getCurrentRepo()
		if err != nil {
			return "", "", 0, nil, "", fmt.Errorf("failed to get repository: %w", err)
		}
	}

	// Validate repository name
	if err := validateRepositoryName(repository); err != nil {
		return "", "", 0, nil, "", err
	}

	// Parse owner/repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return "", "", 0, nil, "", fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repoName = parts[0], parts[1]

	return owner, repoName, pr, config, configFile, nil
}

// processBatchItems handles verbose output and dry run logic
// Returns true if this is a dry run (caller should exit), false otherwise
func processBatchItems(config *BatchConfig, configFile string, pr int) bool {
	if verbose {
		repository := repo
		if config.Repo != "" {
			repository = config.Repo
		}
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("PR: %d\n", pr)
		fmt.Printf("Config file: %s\n", configFile)
		fmt.Printf("Comments to process: %d\n", len(config.Comments))
		if config.Review != nil {
			fmt.Printf("Review event: %s\n", config.Review.Event)
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would process %d comments from %s on PR #%d:\n", len(config.Comments), configFile, pr)
		for i, comment := range config.Comments {
			fmt.Printf("  %d. %s:%s - %s\n", i+1, comment.File, formatLineOrRange(comment), truncateMessage(comment.Message, MessageTruncateLength))
		}
		if config.Review != nil {
			fmt.Printf("Would create review with event: %s\n", config.Review.Event)
		}
		return true // Indicate dry run was performed
	}

	return false
}

// handleBatchResults executes the final batch processing
func handleBatchResults(client github.GitHubAPI, owner, repoName string, pr int, config *BatchConfig) error {
	return processBatchComments(client, owner, repoName, pr, config)
}

func readBatchConfig(configFile string) (*BatchConfig, error) {
	// Read file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", configFile, err)
	}

	// Parse YAML
	var config BatchConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate configuration
	if len(config.Comments) == 0 && config.Review == nil {
		return nil, fmt.Errorf("configuration must contain either comments or review")
	}

	// Validate comments
	for i, comment := range config.Comments {
		if comment.Message == "" {
			return nil, fmt.Errorf("comment %d: message is required", i+1)
		}

		// Determine comment type (default to review)
		commentType := comment.Type
		if commentType == "" {
			commentType = "review"
		}

		// Validate based on comment type
		if commentType == "review" {
			// Review comments require file and line/range
			if comment.File == "" {
				return nil, fmt.Errorf("comment %d: file is required for review comments", i+1)
			}
			if comment.Line == 0 && comment.Range == "" {
				return nil, fmt.Errorf("comment %d: either line or range is required for review comments", i+1)
			}
			if comment.Line != 0 && comment.Range != "" {
				return nil, fmt.Errorf("comment %d: cannot specify both line and range", i+1)
			}
		} else if commentType == "issue" {
			// Issue comments don't require file or line
			// They're general PR discussion comments
		} else {
			return nil, fmt.Errorf("comment %d: type must be 'review' or 'issue'", i+1)
		}
	}

	// Validate review if present
	if config.Review != nil {
		if config.Review.Event != "" {
			validEvents := []string{"APPROVE", "REQUEST_CHANGES", "COMMENT"}
			isValid := false
			for _, validEvent := range validEvents {
				if config.Review.Event == validEvent {
					isValid = true
					break
				}
			}
			if !isValid {
				return nil, fmt.Errorf("review event must be one of: %s", strings.Join(validEvents, ", "))
			}
		}
	}

	return &config, nil
}

func processBatchComments(client github.GitHubAPI, owner, repo string, pr int, config *BatchConfig) error {
	// If we have a review configuration, create the review with comments
	if config.Review != nil {
		return processAsReview(client, owner, repo, pr, config)
	}

	// Otherwise, process comments individually
	return processIndividualComments(client, owner, repo, pr, config.Comments)
}

func processAsReview(client github.GitHubAPI, owner, repo string, pr int, config *BatchConfig) error {
	// Validate and prepare review comments
	reviewComments, err := validateReviewComments(client, owner, repo, pr, config)
	if err != nil {
		return err
	}

	// Build the review input structure
	reviewInput := buildReviewInput(config, reviewComments)

	// Submit the review and handle results
	return submitReviewWithComments(client, owner, repo, pr, reviewInput, len(reviewComments))
}

// validateReviewComments validates review body and converts comments to review comment format
func validateReviewComments(client github.GitHubAPI, owner, repo string, pr int, config *BatchConfig) ([]github.ReviewCommentInput, error) {
	// Validate review body if present
	if config.Review != nil && config.Review.Body != "" {
		if err := validateCommentBody(config.Review.Body); err != nil {
			return nil, fmt.Errorf("review body validation failed: %w", err)
		}
	}

	// Convert comments to review comment format
	var reviewComments []github.ReviewCommentInput

	for _, comment := range config.Comments {
		// Validate comment message
		if err := validateCommentBody(comment.Message); err != nil {
			return nil, fmt.Errorf("comment validation failed: %w", err)
		}

		// Validate file path if present
		if comment.File != "" {
			if err := validateFilePath(comment.File); err != nil {
				return nil, fmt.Errorf("file path validation failed: %w", err)
			}
		}

		if comment.Type == "issue" {
			// Issue comments can't be part of a review, process separately
			if verbose {
				fmt.Printf("Processing issue comment separately: %s:%s\n", comment.File, formatLineOrRange(comment))
			}
			_, err := client.CreateIssueComment(owner, repo, pr, comment.Message)
			if err != nil {
				return nil, fmt.Errorf("failed to create issue comment: %w", err)
			}
			continue
		}

		// Note: GitHub automatically uses the latest commit SHA for review comments

		// Create review comment input
		reviewComment := github.ReviewCommentInput{
			Body: expandSuggestions(comment.Message),
			Path: comment.File,
			Side: "RIGHT", // Default to RIGHT side (additions/new lines)
		}

		// Set line or range
		if comment.Range != "" {
			startLine, endLine, err := parseRange(comment.Range)
			if err != nil {
				return nil, fmt.Errorf("invalid range %s: %w", comment.Range, err)
			}
			reviewComment.StartLine = startLine
			reviewComment.Line = endLine
		} else {
			reviewComment.Line = comment.Line
		}

		// Validate line exists in diff if validation is enabled
		if validateDiff {
			if err := validateCommentLine(client, owner, repo, pr, reviewComment); err != nil {
				return nil, fmt.Errorf("review comment validation failed: %w", err)
			}
		}

		reviewComments = append(reviewComments, reviewComment)
	}

	return reviewComments, nil
}

// buildReviewInput creates the ReviewInput structure with default values
func buildReviewInput(config *BatchConfig, reviewComments []github.ReviewCommentInput) github.ReviewInput {
	reviewInput := github.ReviewInput{
		Body:     config.Review.Body,
		Event:    config.Review.Event,
		Comments: reviewComments,
	}

	if reviewInput.Event == "" {
		reviewInput.Event = "COMMENT"
	}

	return reviewInput
}

// submitReviewWithComments submits the review and handles success reporting
func submitReviewWithComments(client github.GitHubAPI, owner, repo string, pr int, reviewInput github.ReviewInput, commentCount int) error {
	err := client.CreateReview(owner, repo, pr, reviewInput)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	fmt.Printf("%s\n", ColorizeSuccess(fmt.Sprintf("Successfully created review with %d %s", commentCount, pluralize("comment", commentCount))))

	// Show what each comment landed on (see printCommentAnchors).
	printCommentAnchors(client, owner, repo, pr, reviewInput.Comments)

	return nil
}

func processIndividualComments(client github.GitHubAPI, owner, repo string, pr int, comments []CommentConfig) error {
	successCount := 0

	// Group comments by type
	var issueComments []CommentConfig
	var reviewComments []CommentConfig

	for _, comment := range comments {
		commentType := comment.Type
		if commentType == "" {
			commentType = "review" // Default to review comments
		}

		if commentType == "issue" {
			issueComments = append(issueComments, comment)
		} else {
			reviewComments = append(reviewComments, comment)
		}
	}

	// Process issue comments individually
	for i, comment := range issueComments {
		// Validate comment message
		if err := validateCommentBody(comment.Message); err != nil {
			return fmt.Errorf("issue comment %d validation failed: %w", i+1, err)
		}

		if verbose {
			fmt.Printf("Processing issue comment %d/%d: general discussion\n", i+1, len(issueComments))
		}

		_, err := client.CreateIssueComment(owner, repo, pr, expandSuggestions(comment.Message))
		if err != nil {
			return fmt.Errorf("failed to create issue comment %d: %w", i+1, err)
		}

		successCount++
	}

	// Process review comments as a single review (if any)
	if len(reviewComments) > 0 {
		// Convert to ReviewInput format
		var reviewCommentInputs []github.ReviewCommentInput

		for i, comment := range reviewComments {
			// Validate comment message
			if err := validateCommentBody(comment.Message); err != nil {
				return fmt.Errorf("review comment %d validation failed: %w", i+1, err)
			}

			// Validate file path if present
			if comment.File != "" {
				if err := validateFilePath(comment.File); err != nil {
					return fmt.Errorf("review comment %d file path validation failed: %w", i+1, err)
				}
			}

			if verbose {
				fmt.Printf("Processing review comment %d/%d: %s:%s\n", i+1, len(reviewComments), comment.File, formatLineOrRange(comment))
			}

			reviewComment := github.ReviewCommentInput{
				Body: expandSuggestions(comment.Message),
				Path: comment.File,
				Side: "RIGHT", // Default to RIGHT side (additions/new lines)
			}

			// Set line or range
			if comment.Range != "" {
				startLine, endLine, err := parseRange(comment.Range)
				if err != nil {
					return fmt.Errorf("invalid range %s: %w", comment.Range, err)
				}
				reviewComment.StartLine = startLine
				reviewComment.Line = endLine
			} else {
				reviewComment.Line = comment.Line
			}

			// Validate line exists in diff if validation is enabled
			if validateDiff {
				if err := validateCommentLine(client, owner, repo, pr, reviewComment); err != nil {
					return fmt.Errorf("review comment %d validation failed: %w", i+1, err)
				}
			}

			reviewCommentInputs = append(reviewCommentInputs, reviewComment)
		}

		// Create the review with all comments
		reviewInput := github.ReviewInput{
			Body:     "", // No review body for individual comments
			Event:    "COMMENT",
			Comments: reviewCommentInputs,
		}

		err := client.CreateReview(owner, repo, pr, reviewInput)
		if err != nil {
			return fmt.Errorf("failed to create review with %d comments: %w", len(reviewComments), err)
		}

		// This is the second path that creates a review with inline comments
		// (the configured-review path goes through submitReviewWithComments).
		// It needs the anchors for the same reason that one does.
		printCommentAnchors(client, owner, repo, pr, reviewCommentInputs)

		successCount += len(reviewComments)
	}

	fmt.Printf("%s\n", ColorizeSuccess(fmt.Sprintf("Successfully created %d comments", successCount)))
	return nil
}

// Helper functions
func formatLineOrRange(comment CommentConfig) string {
	if comment.Range != "" {
		return comment.Range
	}
	return fmt.Sprintf("%d", comment.Line)
}

func truncateMessage(message string, maxLen int) string {
	if len(message) <= maxLen {
		return message
	}
	return message[:maxLen-TruncationReserve] + TruncationSuffix
}

func parseRange(rangeStr string) (startLine, endLine int, err error) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("range must be in format 'start-end'")
	}

	startLine, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start line: %w", err)
	}

	endLine, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end line: %w", err)
	}

	if startLine <= 0 || endLine <= 0 {
		return 0, 0, fmt.Errorf("line numbers must be positive")
	}

	if startLine > endLine {
		return 0, 0, fmt.Errorf("start line (%d) cannot be greater than end line (%d)", startLine, endLine)
	}

	return startLine, endLine, nil
}
