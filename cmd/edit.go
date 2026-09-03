package cmd

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	editMessages []string

	// Client for dependency injection (tests can override)
	editClient github.GitHubAPI
)

var editCmd = &cobra.Command{
	Use:   "edit <comment-id> [message]",
	Short: "Edit an existing comment on a PR",
	Long: heredoc.Doc(`
		Edit an existing comment on a pull request.

		You can edit with a new message using either positional argument or --message flags.
		Use the comment ID from the URL shown in 'gh comment list' output.

		Common use cases:
		- Fix typos in comments: "Fixed typo in previous comment"
		- Add more context: "Adding more details about the implementation"
		- Refine AI-generated comments: "Updating comment based on new analysis"
		- Correct mistakes: "Correcting the suggested approach"
	`),
	Example: heredoc.Doc(`
		# Edit with new message
		$ gh comment edit 2246362251 "Updated comment with better explanation"

		# Edit with multi-line content using --message flags (AI-friendly)
		$ gh comment edit 2246362251 --message "First paragraph" --message "Second paragraph"

		# Edit with multi-line content (shell native)
		$ gh comment edit 2246362251 "Line 1
		Line 2
		Line 3"
	`),
	Args: cobra.RangeArgs(1, 2),
	RunE: runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
	editCmd.Flags().StringArrayVarP(&editMessages, "message", "m", []string{}, "Edit message (can be used multiple times for multi-line comments)")
}

func runEdit(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if editClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		editClient = client
	}

	// Parse comment ID
	commentID, err := parsePositiveInt(args[0], "comment ID")
	if err != nil {
		return err
	}

	var message string

	// Handle message from positional arg or --message flags
	if len(args) == 2 {
		message = args[1]
	} else if len(editMessages) > 0 {
		message = strings.Join(editMessages, "\n")
	} else {
		return fmt.Errorf("must provide either a message argument or --message flags")
	}

	// Validate comment body length
	if err := validateCommentBody(message); err != nil {
		return err
	}

	// Get repository context. The comment ID identifies its own PR, so this
	// works off-branch.
	repository, prNumber, err := getPRContextForComment(editClient, commentID)
	if err != nil {
		return err
	}

	// Validate repository name
	if err := validateRepositoryName(repository); err != nil {
		return err
	}

	owner, repoName, err := splitRepo(repository)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("Comment ID: %d\n", commentID)
		fmt.Printf("New message: %s\n", message)
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would edit comment #%d:\n", commentID)
		fmt.Printf("New message: %s\n", message)
		return nil
	}

	// Edit the comment using the client
	err = editClient.EditComment(owner, repoName, commentID, prNumber, message)
	if err != nil {
		return formatActionableError("comment editing", err)
	}

	fmt.Printf("%s\n", ColorizeSuccess(fmt.Sprintf("Edited comment #%d", commentID)))
	return nil
}
