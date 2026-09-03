package cmd

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	// Client for dependency injection (tests can override)
	resolveClient github.GitHubAPI
)

var resolveCmd = &cobra.Command{
	Use:   "resolve <comment-id>",
	Short: "Resolve a conversation thread",
	Long: heredoc.Doc(`
		Resolve a conversation thread for a pull request review comment.

		This marks the conversation as resolved, indicating that the feedback
		has been addressed. Use the comment ID from 'gh comment list' output.

		Only review comments can be resolved - issue comments cannot be resolved
		as they don't create conversation threads.
	`),
	Example: heredoc.Doc(`
		# Resolve a conversation
		$ gh comment resolve 2246362251

		# Resolve with dry-run preview
		$ gh comment resolve --dry-run 2246362251

		# Resolve multiple conversations (pipe from list)
		$ gh comment list 123 --type review --ids-only | xargs -I {} gh comment resolve {}
	`),
	Args: cobra.ExactArgs(1),
	RunE: runResolve,
}

func init() {
	rootCmd.AddCommand(resolveCmd)
}

func runResolve(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if resolveClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		resolveClient = client
	}

	// Parse comment ID
	commentID, err := parsePositiveInt(args[0], "comment ID")
	if err != nil {
		return err
	}

	// Get repository and PR context. The comment ID identifies its own PR, so
	// this works off-branch.
	repository, pr, err := getPRContextForComment(resolveClient, commentID)
	if err != nil {
		return err
	}

	owner, repoName, err := splitRepo(repository)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("PR Number: %d\n", pr)
		fmt.Printf("Comment ID: %d\n", commentID)
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would resolve conversation for comment #%d in PR #%d\n", commentID, pr)
		return nil
	}

	// Find the review thread for this comment
	threadID, err := resolveClient.FindReviewThreadForComment(owner, repoName, pr, commentID)
	if err != nil {
		return fmt.Errorf("failed to find review thread for comment: %w", err)
	}

	// Resolve the review thread
	err = resolveClient.ResolveReviewThread(threadID)
	if err != nil {
		return fmt.Errorf("failed to resolve conversation: %w", err)
	}

	fmt.Printf("%s\n", ColorizeSuccess(fmt.Sprintf("Resolved conversation for comment #%d", commentID)))
	return nil
}
