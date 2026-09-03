package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/markusmobius/go-dateparser"
	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	author      string
	quiet       bool
	hideAuthors bool

	// Filtering flags
	showRecent bool   // Show only comments from last 7 days
	filter     string // "today" or custom filter
	since      string
	until      string
	listType   string
	listStatus string // "open", "resolved", or "all"

	// Output format flags
	outputFormat string
	idsOnly      bool

	// Parsed time values
	sinceTime *time.Time
	untilTime *time.Time

	// Client for dependency injection (tests can override)
	listClient github.GitHubAPI
)

var listCmd = &cobra.Command{
	Use:   "list [pr]",
	Short: "List comments with advanced filtering and formatting options",
	Long: heredoc.Doc(`
		List all comments on a pull request sorted by newest first.
		By default, shows all comments. Use --recent to see only the last 7 days.

		Comment Types:
		- Issue comments: General PR discussion, appear in main conversation tab
		- Review comments: Line-specific feedback, appear in "Files Changed" tab

		Comments can be filtered by type, author, date range, and resolution status.

		Resolution status comes from review threads, so only review comments can
		be resolved. Issue comments have no thread and always count as open.

		Output can be formatted as tables, JSON, or plain text with color coding.
		Perfect for code review workflows, comment analysis, and automation.
	`),
	Example: heredoc.Doc(`
		# Basic usage - list all comments (newest first)
		$ gh comment list 123

		# Show only recent comments (last 7 days)
		$ gh comment list 123 --recent

		# Review team analysis and metrics
		$ gh comment list 123 --author "senior-dev*" --recent
		$ gh comment list 123 --type review --author "*@company.com" --since "2024-01-01"

		# Security audit and compliance tracking
		$ gh comment list 123 --author "security-team*" --since "2024-01-01" --type review
		$ gh comment list 123 --author "bot*" --recent --quiet

		# Structured output for automation
		$ gh comment list 123 --format json | jq '.comments[].id'
		$ gh comment list 123 --ids-only | xargs -I {} gh comment resolve {}
		$ gh comment list 123 --format json --author "security*" > security-comments.json

		# Code review workflow optimization
		$ gh comment list 123 --since "1 month ago" --author "lead*"
		$ gh comment list 123 --until "2024-12-31" --type issue

		# Team communication patterns
		$ gh comment list 123 --author "qa*" --since "3 days ago" --type review
		$ gh comment list 123 --author "*@contractor.com" --since "1 month ago"

		# Blocker identification and recent activity
		$ gh comment list 123 --author "architect*" --recent --type review
		$ gh comment list 123 --since "critical-bug-report" --author "oncall*"

		# Performance review analysis
		$ gh comment list 123 --author "performance-team" --since "load-test-date" --type review
		$ gh comment list 123 --recent --author "*perf*"

		# Export for further analysis and automation
		$ gh comment list 123 --author "all-reviewers*" --since "quarter-start" --quiet | process-review-data.sh
		$ gh comment list 123 --ids-only --type review --recent | review-metrics.sh
	`),
	Args:   cobra.MaximumNArgs(1),
	PreRun: applyListConfigDefaults,
	RunE:   runList,
}

func init() {
	// Filter flags
	listCmd.Flags().BoolVar(&showRecent, "recent", false, "Show only comments from last 7 days")
	listCmd.Flags().StringVar(&filter, "filter", "", "Filter comments (today)")
	listCmd.Flags().StringVar(&author, "author", "", "Filter by author (supports wildcards: 'alice*', '*@company.com')")
	listCmd.Flags().StringVar(&since, "since", "", "Show comments after this date/time (flexible formats)")
	listCmd.Flags().StringVar(&until, "until", "", "Show comments before this date/time (flexible formats)")
	listCmd.Flags().StringVar(&listType, "type", "", "Filter by type (issue|review)")
	listCmd.Flags().StringVar(&listStatus, "status", "all", "Filter by resolution status (open|resolved|all)")

	// Display flags
	listCmd.Flags().BoolVar(&quiet, "quiet", false, "Minimal output (hides URLs and formatting)")
	listCmd.Flags().BoolVar(&hideAuthors, "hide-authors", false, "Hide comment authors in output")

	// Output format flags
	listCmd.Flags().StringVar(&outputFormat, "format", "default", "Output format (default|json)")
	listCmd.Flags().BoolVar(&idsOnly, "ids-only", false, "Output only comment IDs (one per line)")

	// Register command
	rootCmd.AddCommand(listCmd)
}

// applyListConfigDefaults applies configuration defaults to list command flags
func applyListConfigDefaults(cmd *cobra.Command, args []string) {
	config := GetConfig()

	// Apply filter defaults if flags weren't explicitly set
	if !cmd.Flags().Changed("author") && config.Defaults.Author != "" {
		author = config.Defaults.Author
	}
	if !cmd.Flags().Changed("recent") && config.Filters.Status != "" {
		// Map legacy status config to new --recent flag
		switch config.Filters.Status {
		case "recent", "unresolved", "pending":
			// Show recent comments
			showRecent = true
			// case "all": default behavior, show all
		}
	}
	if !cmd.Flags().Changed("type") && config.Filters.Type != "" {
		listType = config.Filters.Type
	}
	if !cmd.Flags().Changed("since") && config.Filters.Since != "" {
		since = config.Filters.Since
	}
	if !cmd.Flags().Changed("until") && config.Filters.Until != "" {
		until = config.Filters.Until
	}

	// Apply display defaults
	if !cmd.Flags().Changed("format") && config.Display.Format != "table" {
		// Map config format to list command format
		switch config.Display.Format {
		case "json":
			outputFormat = "json"
		case "quiet":
			quiet = true
		}
	}
	if !cmd.Flags().Changed("quiet") && config.Display.Quiet {
		quiet = config.Display.Quiet
	}
}

func runList(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if listClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to initialize GitHub client: %w", err)
		}
		listClient = client
	}

	// Parse and validate command arguments
	repository, pr, err := parseListArguments(args)
	if err != nil {
		return err
	}

	// Fetch and filter comments based on criteria
	filteredComments, err := fetchAndFilterComments(listClient, repository, pr)
	if err != nil {
		return err
	}

	// Format and display the output
	return formatListOutput(filteredComments, pr)
}

// parseListArguments handles validation and parsing of command arguments and flags
func parseListArguments(args []string) (repository string, pr int, err error) {
	// Validate and parse filtering flags
	if err := validateAndParseFilters(); err != nil {
		return "", 0, err
	}

	// Parse PR argument using centralized function
	if len(args) == 1 {
		pr, err = parsePositiveInt(args[0], "PR number")
		if err != nil {
			return "", 0, err
		}
		// Get repository for explicitly provided PR
		repository, err = getCurrentRepo()
		if err != nil {
			return "", 0, err
		}
	} else {
		// Auto-detect PR and repository using centralized function
		repository, pr, err = getPRContext()
		if err != nil {
			return "", 0, err
		}
	}

	return repository, pr, nil
}

// fetchAndFilterComments handles verbose output, comment fetching, and filtering
func fetchAndFilterComments(client github.GitHubAPI, repository string, pr int) ([]Comment, error) {
	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("PR: %d\n", pr)
		fmt.Printf("Filter: %s\n", filter)
		fmt.Printf("Quiet mode: %v\n", quiet)
		fmt.Printf("Hide authors: %v\n", hideAuthors)
		if author != "" {
			fmt.Printf("Filter by author: %s\n", author)
		}
		if listType != "" {
			fmt.Printf("Filter by type: %s\n", listType)
		}
		if sinceTime != nil {
			fmt.Printf("Since: %s\n", sinceTime.Format(time.RFC3339))
		}
		if untilTime != nil {
			fmt.Printf("Until: %s\n", untilTime.Format(time.RFC3339))
		}
		fmt.Println()
	}

	// Fetch comments
	comments, err := fetchAllComments(client, repository, pr)
	if err != nil {
		return nil, err
	}

	// Filter comments
	filteredComments := filterComments(comments)

	// Sort by newest first
	sortCommentsByNewest(filteredComments)

	return filteredComments, nil
}

// formatListOutput handles different output formats and display
func formatListOutput(filteredComments []Comment, pr int) error {
	// Handle different output formats
	if idsOnly {
		displayIDsOnly(filteredComments)
	} else if outputFormat == "json" {
		if err := displayCommentsJSON(filteredComments, pr); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
	} else {
		displayComments(filteredComments, pr)
	}

	return nil
}

type Comment struct {
	ID        int       `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// For line-specific comments
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	CommitID string `json:"commit_id,omitempty"`

	// Comment type
	Type string `json:"type"` // "issue" or "review"

	// Resolution state of the review thread this comment belongs to.
	// Issue comments have no thread and are therefore never resolved.
	Resolved bool `json:"resolved"`
}

func fetchAllComments(client github.GitHubAPI, repo string, pr int) ([]Comment, error) {
	// Parse owner/repo
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repo)
	}
	owner := parts[0]
	repoName := parts[1]

	var allComments []Comment

	// Fetch issue comments (general discussion)
	issueComments, err := client.ListIssueComments(owner, repoName, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issue comments: %w", err)
	}

	for _, comment := range issueComments {
		allComments = append(allComments, Comment{
			ID:        comment.ID,
			Author:    comment.User.Login,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Type:      "issue",
		})
	}

	// Fetch review comments (line-specific)
	reviewComments, err := client.ListReviewComments(owner, repoName, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch review comments: %w", err)
	}

	// Resolution state lives on review threads in GraphQL, not on the REST
	// comment payloads, so it costs an extra query. Only pay for it when a
	// filter actually asks — the default listing does not.
	resolved := map[int]bool{}
	if listStatus == "open" || listStatus == "resolved" {
		threads, err := client.ListReviewThreads(owner, repoName, pr)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch review threads (needed for --status): %w", err)
		}
		for _, thread := range threads {
			for _, id := range thread.CommentIDs {
				resolved[id] = thread.IsResolved
			}
		}
	}

	for _, comment := range reviewComments {
		allComments = append(allComments, Comment{
			ID:        comment.ID,
			Author:    comment.User.Login,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			UpdatedAt: comment.UpdatedAt,
			Path:      comment.Path,
			Line:      comment.Line,
			CommitID:  comment.CommitID,
			Type:      "review",
			// A comment in no thread we know of counts as open, which is also
			// what a comment on a PR with no threads at all should read as.
			Resolved: resolved[comment.ID],
		})
	}

	// Sort by creation time
	for i := 0; i < len(allComments)-1; i++ {
		for j := i + 1; j < len(allComments); j++ {
			if allComments[i].CreatedAt.After(allComments[j].CreatedAt) {
				allComments[i], allComments[j] = allComments[j], allComments[i]
			}
		}
	}

	return allComments, nil
}

func validateAndParseFilters() error {
	// Validate filter flag
	validFilters := []string{"", "today"}
	if filter != "" && !containsString(validFilters, filter) {
		return fmt.Errorf("invalid filter '%s'. Must be one of: today", filter)
	}

	// Apply filter-based date defaults
	now := time.Now()

	// Handle --recent flag (last 7 days)
	if showRecent && since == "" && sinceTime == nil {
		sevenDaysAgo := now.AddDate(0, 0, -7)
		sinceTime = &sevenDaysAgo
	}

	// Handle --filter today
	if filter == "today" {
		// Comments from today only
		if since == "" && sinceTime == nil {
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			sinceTime = &startOfDay
		}
	}

	// Validate comment type flag
	validTypes := []string{"", "issue", "review"}
	if listType != "" && !containsString(validTypes, listType) {
		return fmt.Errorf("invalid type '%s'. Must be one of: issue, review", listType)
	}

	// Validate resolution status flag
	validStatuses := []string{"", "open", "resolved", "all"}
	if !containsString(validStatuses, listStatus) {
		return fmt.Errorf("invalid status '%s'. Must be one of: open, resolved, all", listStatus)
	}

	// Validate output format flag
	validFormats := []string{"default", "json"}
	if outputFormat != "" && !containsString(validFormats, outputFormat) {
		return fmt.Errorf("invalid format '%s'. Must be one of: %s", outputFormat, strings.Join(validFormats, ", "))
	}

	// Handle conflicting output options
	if idsOnly && outputFormat == "json" {
		return fmt.Errorf("cannot use --ids-only with --format json (use --format json to get structured data including IDs)")
	}

	// Parse explicit since date (overrides filter defaults)
	if since != "" {
		parsedTime, err := parseFlexibleDate(since)
		if err != nil {
			return fmt.Errorf("invalid since date '%s': %w", since, err)
		}
		sinceTime = &parsedTime
	}

	// Parse until date
	if until != "" {
		parsedTime, err := parseFlexibleDate(until)
		if err != nil {
			return fmt.Errorf("invalid until date '%s': %w", until, err)
		}
		untilTime = &parsedTime
	}

	// Validate date range
	if sinceTime != nil && untilTime != nil && sinceTime.After(*untilTime) {
		return fmt.Errorf("since date (%s) cannot be after until date (%s)", since, until)
	}

	return nil
}

func parseFlexibleDate(dateStr string) (time.Time, error) {
	// Try parsing with dateparser for flexible formats
	// This handles things like "yesterday", "3 days ago", "2024-01-01", etc.
	parsed, err := dateparser.Parse(&dateparser.Configuration{
		CurrentTime: time.Now(),
	}, dateStr)

	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse date: %w", err)
	}

	return parsed.Time, nil
}

func filterComments(comments []Comment) []Comment {
	var filtered []Comment

	for _, comment := range comments {
		// Filter by author (supports wildcards)
		if author != "" && !matchesAuthorFilter(comment.Author, author) {
			continue
		}

		// Filter by comment type
		if listType != "" && comment.Type != listType {
			continue
		}

		// Filter by resolution status. Only review comments live in threads,
		// so an issue comment is never resolved and always counts as open.
		if listStatus == "open" && comment.Resolved {
			continue
		}
		if listStatus == "resolved" && !comment.Resolved {
			continue
		}

		// Filter by date range (already set based on filter flag in validateAndParseFilters)
		if sinceTime != nil && comment.CreatedAt.Before(*sinceTime) {
			continue
		}
		if untilTime != nil && comment.CreatedAt.After(*untilTime) {
			continue
		}

		filtered = append(filtered, comment)
	}

	return filtered
}

func matchesAuthorFilter(author, filter string) bool {
	// Exact match
	if author == filter {
		return true
	}

	// Wildcard matching
	if strings.Contains(filter, "*") {
		// Convert wildcard pattern to regex
		pattern := strings.ReplaceAll(regexp.QuoteMeta(filter), `\*`, `.*`)
		pattern = "^" + pattern + "$"

		if matched, err := regexp.MatchString(pattern, author); err == nil && matched {
			return true
		}
	}

	// Case-insensitive partial match
	return strings.Contains(strings.ToLower(author), strings.ToLower(filter))
}

// sortCommentsByNewest sorts comments with newest first
func sortCommentsByNewest(comments []Comment) {
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
}

// filterRecentComments filters comments to only include those from the last N days
func filterRecentComments(comments []Comment, days int) []Comment {
	cutoff := time.Now().AddDate(0, 0, -days)
	var filtered []Comment

	for _, comment := range comments {
		if comment.CreatedAt.After(cutoff) {
			filtered = append(filtered, comment)
		}
	}

	return filtered
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func displayComments(comments []Comment, pr int) {
	if len(comments) == 0 {
		fmt.Printf("No comments found on PR #%d\n", pr)
		return
	}

	// Collect unique commit IDs for summary
	commitIDs := make(map[string]bool)
	for _, comment := range comments {
		if comment.CommitID != "" {
			commitIDs[comment.CommitID] = true
		}
	}

	fmt.Printf("📝 Comments on PR #%d (%d total", pr, len(comments))
	if len(commitIDs) > 0 {
		fmt.Printf(", %d commit", len(commitIDs))
		if len(commitIDs) != 1 {
			fmt.Printf("s")
		}
	}
	fmt.Printf(")\n\n")

	for _, comment := range comments {
		displayComment(comment)
	}
}

func displayComment(comment Comment) {
	// Format timestamp
	timeAgo := formatTimeAgo(comment.CreatedAt)

	// Define color functions
	colorAuthor := func(text string) string {
		if ColorAuthor != nil {
			return ColorAuthor.Sprint(text)
		}
		return text
	}
	colorTimestamp := func(text string) string {
		if ColorTimestamp != nil {
			return ColorTimestamp.Sprint(text)
		}
		return text
	}
	colorType := func(text string) string {
		if ColorCommentID != nil {
			return ColorCommentID.Sprint(text)
		}
		return text
	}

	// Display comment header with type indicator
	typeIndicator := ""
	if comment.Type == "review" {
		typeIndicator = "[📋 Review]"
		if comment.Path != "" && comment.Line > 0 {
			typeIndicator = fmt.Sprintf("[📋 Review: %s:%d]",
				comment.Path, comment.Line)
		}
	} else {
		typeIndicator = "[💬 Issue]"
	}

	// Build header components with ID
	idStr := fmt.Sprintf("ID:%d", comment.ID)
	var headerComponents []string
	headerComponents = append(headerComponents, idStr)
	if !hideAuthors {
		headerComponents = append(headerComponents, colorAuthor(comment.Author))
	}
	headerComponents = append(headerComponents, colorTimestamp(timeAgo))
	headerComponents = append(headerComponents, colorType(typeIndicator))

	// Print header
	fmt.Printf("🔹 %s\n", strings.Join(headerComponents, " • "))

	// Process and display comment body
	body := strings.TrimSpace(comment.Body)
	if body == "" {
		body = "(empty comment)"
	}

	// Indent the comment body
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		fmt.Printf("   %s\n", line)
	}

	// Note: GitHub API doesn't provide HTML URL for comments directly
	// We would need to construct it from PR URL and comment ID

	fmt.Println()
}

func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		} else if days < 7 {
			return fmt.Sprintf("%d days ago", days)
		} else {
			return t.Format("Jan 2, 2006")
		}
	}
}

func displayDiffHunk(diffHunk string) {
	// Split diff hunk into lines
	lines := strings.Split(strings.TrimSpace(diffHunk), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Color code the diff lines
		if strings.HasPrefix(line, "@@") {
			// Diff header - show line numbers
			fmt.Printf("   🔹 %s\n", line)
		} else if strings.HasPrefix(line, "+") {
			// Added line - green
			fmt.Printf("   ➕ %s\n", line)
		} else if strings.HasPrefix(line, "-") {
			// Removed line - red
			fmt.Printf("   ➖ %s\n", line)
		} else {
			// Context line - neutral
			fmt.Printf("     %s\n", line)
		}
	}
	fmt.Println()
}

// displayIDsOnly outputs only comment IDs, one per line
func displayIDsOnly(comments []Comment) {
	for _, comment := range comments {
		fmt.Printf("%d\n", comment.ID)
	}
}

// displayCommentsJSON outputs comments as JSON
func displayCommentsJSON(comments []Comment, pr int) error {
	output := struct {
		PR       int       `json:"pr"`
		Total    int       `json:"total"`
		Comments []Comment `json:"comments"`
	}{
		PR:       pr,
		Total:    len(comments),
		Comments: comments,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
