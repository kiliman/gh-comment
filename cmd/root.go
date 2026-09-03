package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/go-gh/v2"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	prNumber     int
	repo         string
	validateDiff bool
	dryRun       bool
	verbose      bool
	configPath   string
	noColor      bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gh-comment",
	Short: "Strategic GitHub PR commenting and review management",
	Long: heredoc.Doc(`
		gh-comment provides professional-grade tools for GitHub pull request
		comment management, systematic code reviews, and review process automation.

		Designed for developers, code review leads, and teams who need sophisticated
		comment workflows beyond GitHub's web interface capabilities.

		Comment Types:
		• Issue comments: General PR discussion, appear in main conversation tab
		• Review comments: Line-specific feedback, appear in "Files Changed" tab

		Strategic GitHub PR Commenting - Beyond the Web Interface:

		gh-comment is designed for professional code review workflows that require:
		• Systematic line-by-line code analysis with context
		• Bulk comment operations for comprehensive reviews
		• Advanced filtering for comment analysis and metrics
		• Automation integration for CI/CD review processes
		• Data export for review process optimization

		Perfect for: Senior developers, code review leads, QA teams, and DevOps engineers
		who need sophisticated comment management beyond GitHub's web interface.
	`),
	Example: heredoc.Doc(`
		Commands:
		  add                     Add general PR discussion comments
		  batch                   Process multiple comments from YAML configuration
		  close-pending-review    Submit GUI-created pending reviews
		  edit                    Modify existing comments
		  lines                   Show commentable lines in PR files
		  list                    List and filter comments with advanced options
		  prompts                 Get AI-powered code review prompts and best practices
		  react                   Add or remove emoji reactions to comments
		  resolve                 Resolve conversation threads
		  review                  Create line-specific code reviews
		  review-reply            Reply to review comments with text messages
		  help                    Help about any command

		Global Flags:
		  -p, --pr int        PR number (auto-detect from branch if omitted)
		  -R, --repo string   Repository (owner/repo format)
		      --dry-run       Show what would be commented without executing
		  -v, --verbose       Show detailed API interactions
		      --validate      Validate line exists in diff before commenting (default false)

		Filtering Flags (list command):
		      --author string     Filter by author (supports wildcards: 'user*')
		      --since string      Show comments after date ('2024-01-01', '1 week ago')
		      --until string      Show comments before date
		      --type string       Filter by type: issue, review
		      --recent            Show only comments from the last 7 days
		  -q, --quiet            Minimal output for scripts

		Review Flags:
		      --event string      Review event: APPROVE, REQUEST_CHANGES, COMMENT
		      --comment strings   Add comments: file:line:message

		Examples:
		# Basic Operations
		$ gh comment list 123                           List all comments on PR #123
		$ gh comment add 123 "Looks good overall!"     Add general discussion comment

		# Issue Comments (General PR Discussion)
		$ gh comment add 123 "LGTM! Just waiting for CI to pass"
		$ gh comment add 123 "Thanks for addressing the security concerns"
		$ gh comment review-reply 12345 "Good point, I'll make those changes"

		# Review Comments (Line-Specific Code Feedback)
		$ gh comment review 123 "Code review complete" \
		  --comment src/api.js:42:"Add rate limiting middleware" \
		  --comment src/main.go:3:4:"Refactor for OAuth2 compliance" \
		  --event REQUEST_CHANGES

		# Advanced Filtering (Power User Features)
		$ gh comment list 123 --author "senior-dev*" --since "1 week ago"
		$ gh comment list 123 --type review --author "*@company.com" --since "2024-01-01"
		$ gh comment list 123 --type review --until "2024-01-01" --quiet

		# Review Workflows (Professional Code Review)
		$ gh comment review 123 "Migration review complete" \
		  --comment src/api.js:42:"Add rate limiting middleware" \
		  --comment src/auth.js:15:20:"Update to OAuth2 flow" \
		  --comment tests/api_test.go:100:"Add edge case tests" \
		  --event REQUEST_CHANGES

		$ gh comment review 123 "Security audit findings" \
		  --comment src/api.js:8:"Use crypto.randomBytes(32) for tokens" \
		  --comment api.js:134:140:"Extract business logic to service layer"

		# Batch Operations (Systematic Reviews)
		$ gh comment batch 123 review-config.yaml
		$ gh comment batch 456 security-checklist.yaml --dry-run
		$ gh comment batch 789 bulk-comments.yaml --verbose

		# Conversation Management
		$ gh comment review-reply 2246362251 "Fixed in commit abc123" --resolve
		$ gh comment react 3141344022 +1
		$ gh comment react 2246362251 rocket
		$ gh comment react 3141344022 heart --remove
		$ gh comment resolve 2246362251

		# Data Export & Analysis (Automation)
		$ gh comment list 123 --quiet | grep "👤" | cut -d' ' -f2 | sort | uniq -c
		$ gh comment list 123 --since "2024-01-01" --quiet | tee q1-review-data.txt
		$ gh comment list 123 --author "qa-team*" --quiet | analyze-feedback.py

		# Automation & CI Integration with Suggestion Syntax
		$ gh comment add 123 src/security.js 67 "[SUGGEST: use crypto.randomBytes(32)]"
		$ gh comment add 123 src/api.js 42 "[SUGGEST:+2: const timeout = 5000;]"
		$ gh comment add 123 src/utils.js 15 "[SUGGEST:-1: import { validateInput } from './validators';]"
		$ for file in $(git diff --name-only); do gh comment add 123 "$file" 1 "Auto-generated security scan results"; done
		$ gh comment list --since "1 week ago" --type review | review-blocker-analysis.sh

		# Advanced Comment Management
		$ gh comment edit 2246362251 "Updated: This rate limiting logic handles concurrent requests properly"
		$ gh comment list 123 --author "bot*" --quiet | grep "ID:" | cut -d':' -f2 | xargs -I {} gh comment resolve {}
		$ gh comment add 123 performance.js 89:95 "Consider caching this expensive calculation"

		# Suggestion Syntax (Auto-expand to GitHub suggestion blocks):
		Basic syntax:     [SUGGEST: improved_code]
		Offset syntax:    [SUGGEST:+N: code_for_N_lines_below]
		                  [SUGGEST:-N: code_for_N_lines_above]
		Multi-line:       <<<SUGGEST
		                  multi_line_code
		                  SUGGEST>>>

		Examples:
		$ gh comment add 123 src/api.js 42 "[SUGGEST: const timeout = 5000;]"
		$ gh comment add 123 src/api.js 40 "[SUGGEST:+2: // Add error handling]"
		$ gh comment add 123 src/api.js 45 "[SUGGEST:-1: import { logger } from './utils';]"
	`),
	Version: "1.0.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().IntVarP(&prNumber, "pr", "p", 0, "PR number (default: auto-detect from branch)")
	rootCmd.PersistentFlags().StringVarP(&repo, "repo", "R", "", "Repository in owner/repo format (default: auto-detect from current directory)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Configuration file path (default: search standard locations)")

	rootCmd.PersistentFlags().BoolVar(&validateDiff, "validate", false, "Validate line exists in diff before commenting (default: false)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be commented without executing (default: false)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed API interactions (default: false)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output (default: false)")

	// Load configuration and initialize colors before command execution
	cobra.OnInitialize(initializeConfig, initializeColors)
}

// initializeColors initializes the color system
func initializeColors() {
	InitColors()
}

// initializeConfig loads the configuration and applies defaults
func initializeConfig() {
	err := LoadGlobalConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load configuration: %v\n", err)
		return
	}

	config := GetConfig()

	// Apply config defaults if flags weren't explicitly set
	applyConfigDefaults(config)
}

// applyConfigDefaults applies configuration defaults to global variables
func applyConfigDefaults(config *Config) {
	// Only apply defaults if values weren't set via flags
	if repo == "" && config.Defaults.Repository != "" {
		repo = config.Defaults.Repository
	}
	if prNumber == 0 && config.Defaults.PR != 0 {
		prNumber = config.Defaults.PR
	}

	// Apply behavior defaults (these could be overridden by flags)
	if !rootCmd.PersistentFlags().Changed("dry-run") {
		dryRun = config.Behavior.DryRun
	}
	if !rootCmd.PersistentFlags().Changed("verbose") {
		verbose = config.Behavior.Verbose
	}
	if !rootCmd.PersistentFlags().Changed("validate") {
		validateDiff = config.Behavior.Validate
	}
}

// Helper function to get current repository if not specified
func getCurrentRepo() (string, error) {
	if repo != "" {
		return repo, nil
	}

	// Use gh CLI to get current repository
	stdout, _, err := gh.Exec("repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
	if err != nil {
		return "", fmt.Errorf("failed to get current repository: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Helper function to get current PR number if not specified
func getCurrentPR() (int, error) {
	if prNumber != 0 {
		return prNumber, nil
	}

	// Use gh CLI to get PR for current branch
	stdout, _, err := gh.Exec("pr", "view", "--json", "number", "-q", ".number")
	if err != nil {
		return 0, fmt.Errorf("failed to get current PR: %w (try specifying --pr)", err)
	}

	prStr := strings.TrimSpace(stdout.String())
	pr, err := strconv.Atoi(prStr)
	if err != nil {
		return 0, fmt.Errorf("invalid PR number: %s", prStr)
	}

	return pr, nil
}
