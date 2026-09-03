package cmd

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	removeReactionFlag bool

	// Client for dependency injection (tests can override)
	reactClient github.GitHubAPI
)

var reactCmd = &cobra.Command{
	Use:   "react <comment-id> <emoji>",
	Short: "Add or remove emoji reactions to comments",
	Long: heredoc.Doc(`
		Add or remove emoji reactions to any comment type.

		Supports both issue comments and review comments automatically.
		Comment type is auto-detected from the comment ID.

		Available reactions: +1, -1, laugh, confused, heart, hooray, rocket, eyes

		Comment IDs can be found in the output of 'gh comment list'.
	`),
	Example: heredoc.Doc(`
		# Add reactions to any comment type
		$ gh comment react 123456 +1
		$ gh comment react 789012 heart
		$ gh comment react 999999 rocket

		# Remove reactions
		$ gh comment react 123456 +1 --remove
		$ gh comment react 789012 heart --remove

		# Works with both issue and review comments automatically
		$ gh comment react 123456 eyes    # Works for issue comments
		$ gh comment react 789012 hooray  # Works for review comments
	`),
	Args: cobra.ExactArgs(2),
	RunE: runReact,
}

func init() {
	rootCmd.AddCommand(reactCmd)

	reactCmd.Flags().BoolVar(&removeReactionFlag, "remove", false, "Remove reaction instead of adding it")
}

func runReact(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if reactClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		reactClient = client
	}

	// Parse comment ID
	commentIDStr := args[0]
	commentID, err := parsePositiveInt(commentIDStr, "comment ID")
	if err != nil {
		return err
	}

	// Get reaction emoji
	reaction := args[1]

	// Validate reaction
	if !validateReaction(reaction) {
		return formatValidationError("reaction", reaction, "must be one of: +1, -1, laugh, confused, heart, hooray, rocket, eyes")
	}

	// Get repository context. The comment ID identifies its own PR, so this
	// works off-branch.
	repository, prNumber, err := getPRContextForComment(reactClient, commentID)
	if err != nil {
		return err
	}

	owner, repoName, err := splitRepo(repository)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("Comment ID: %d\n", commentID)
		fmt.Printf("Reaction: %s\n", reaction)
		if removeReactionFlag {
			fmt.Printf("Action: Remove reaction\n")
		} else {
			fmt.Printf("Action: Add reaction\n")
		}
		fmt.Println()
	}

	if dryRun {
		action := "add"
		if removeReactionFlag {
			action = "remove"
		}
		fmt.Printf("Would %s %s reaction %s comment #%d\n", action, reaction,
			map[bool]string{true: "from", false: "to"}[removeReactionFlag], commentID)
		return nil
	}

	// Perform the reaction action
	if removeReactionFlag {
		err = reactClient.RemoveReaction(owner, repoName, commentID, prNumber, reaction)
		if err != nil {
			return formatActionableError("reaction removal", err)
		}
		fmt.Printf("✅ Removed %s reaction from comment #%d\n", reaction, commentID)
	} else {
		err = reactClient.AddReaction(owner, repoName, commentID, prNumber, reaction)
		if err != nil {
			return formatActionableError("reaction addition", err)
		}
		fmt.Printf("✅ Added %s reaction to comment #%d\n", reaction, commentID)
	}

	return nil
}

func validateReaction(reaction string) bool {
	validReactions := []string{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"}
	for _, valid := range validReactions {
		if reaction == valid {
			return true
		}
	}
	return false
}
