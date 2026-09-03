package cmd

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"

	"github.com/spf13/cobra"

	"github.com/silouanwright/gh-comment/internal/github"
)

var (
	resolveConversationReviewReply bool
	noExpandSuggestionsReviewReply bool

	// Client for dependency injection (tests can override)
	reviewReplyClient github.GitHubAPI
)

var reviewReplyCmd = &cobra.Command{
	Use:   "review-reply <comment-id> [message]",
	Short: "Reply to a review comment with a text message",
	Long: heredoc.Doc(`
		Reply to an existing review comment with a text message.

		⚠️  **Important GitHub API Limitation**:
		This command only works for "standalone" review comments (added via "Add single comment").
		Comments created through "Start a Review" → "Submit Review" cannot be replied to due to
		GitHub's review threading architecture. This is a known API limitation, not a tool issue.

		When replies fail, the command provides intelligent alternatives:
		• Add new comments at the same location
		• Use emoji reactions for quick feedback
		• Resolve conversations (often works when replies don't)

		For general PR discussion, use 'gh comment add' instead.
		For emoji reactions, use 'gh comment react' command.

		Comment IDs can be found in the output of 'gh comment list'.
	`),
	Example: heredoc.Doc(`
		# Try to reply to review comment (may fail due to API limitations)
		$ gh comment review-reply 789012 "Fixed this issue"

		# Resolve conversation without adding message (more reliable)
		$ gh comment review-reply 789012 --resolve

		# If reply fails, alternatives will be suggested:

		# Alternative 1: Add new comment at same location
		$ gh comment add 123 src/main.go 42 "Fixed this issue"

		# Alternative 2: Use emoji reactions for quick feedback
		$ gh comment react 789012 +1
		$ gh comment react 789012 heart

		# Alternative 3: General PR discussion
		$ gh comment add 123 "Thanks for the review feedback!"

		# Note: When reply fails, the tool provides specific alternatives
		# based on the comment type and failure reason.
	`),
	Args: cobra.RangeArgs(1, 2),
	RunE: runReviewReply,
}

func init() {
	rootCmd.AddCommand(reviewReplyCmd)

	reviewReplyCmd.Flags().BoolVar(&resolveConversationReviewReply, "resolve", false, "Resolve the conversation after replying")
	reviewReplyCmd.Flags().BoolVar(&noExpandSuggestionsReviewReply, "no-expand-suggestions", false, "Disable automatic expansion of [SUGGEST:] and <<<SUGGEST>>> syntax")
}

func runReviewReply(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if reviewReplyClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		reviewReplyClient = client
	}

	// Parse comment ID
	commentIDStr := args[0]
	commentID, err := parsePositiveInt(commentIDStr, "comment ID")
	if err != nil {
		return err
	}

	// Get message if provided
	var message string
	if len(args) > 1 {
		message = args[1]
	}

	// Validate that we have either message or resolve
	if message == "" && !resolveConversationReviewReply {
		return fmt.Errorf("must provide either a message or --resolve")
	}

	// Validate comment body length if provided
	if message != "" {
		if err := validateCommentBody(message); err != nil {
			return err
		}
	}

	// Get repository context. The comment ID identifies its own PR, so this
	// works from any branch — including main, which is where you always are
	// when reviewing someone else's PR.
	repository, prNumber, err := getPRContextForComment(reviewReplyClient, commentID)
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
		fmt.Printf("Comment Type: review\n")
		if message != "" {
			fmt.Printf("Message: %s\n", message)
		}
		if resolveConversationReviewReply {
			fmt.Printf("Resolve Conversation: true\n")
		}
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would reply to review comment #%d:\n", commentID)
		if message != "" {
			fmt.Printf("Message: %s\n", message)
		}
		if resolveConversationReviewReply {
			fmt.Printf("Resolve Conversation: true\n")
		}
		return nil
	}

	// Resolve the thread ID *before* writing anything.
	//
	// This command performs up to two writes and they are not atomic. Posting
	// the reply first meant a failed thread lookup left the reply already
	// committed, with an error that gave no hint of it — and the natural
	// response to an error is to run the command again, which double-posts.
	// Looking up first means nothing is written until both halves are known to
	// be viable, so a failure aborts cleanly and retrying is safe.
	var threadID string
	if resolveConversationReviewReply {
		threadID, err = reviewReplyClient.FindReviewThreadForComment(owner, repoName, prNumber, commentID)
		if err != nil {
			return fmt.Errorf("failed to find review thread for comment #%d in PR #%d: %w\n\n💡 Nothing was posted, so this is safe to retry.\n  • If the PR number looks wrong, pass the right one with --pr\n  • Only review comments live in threads; issue comments cannot be resolved", commentID, prNumber, err)
		}
	}

	// Handle message reply
	replyPosted := false
	if message != "" {
		// Expand suggestions if enabled
		if !noExpandSuggestionsReviewReply {
			message = expandSuggestions(message)
		}

		// Try to reply to review comment with intelligent fallback
		_, err = reviewReplyClient.CreateReviewCommentReply(owner, repoName, commentID, message)
		if err != nil {
			// Enhanced error handling with intelligent fallback suggestions
			return handleReviewReplyError(err, commentID, message, owner, repoName, prNumber)
		}
		replyPosted = true
		fmt.Printf("✅ Replied to review comment #%d: %s\n", commentID, message)
	}

	// Handle resolve conversation
	if resolveConversationReviewReply {
		if err := reviewReplyClient.ResolveReviewThread(threadID); err != nil {
			// The reply is already committed at this point. Say so, and hand
			// over the one-liner that finishes the job, because re-running this
			// command would post the reply a second time.
			if replyPosted {
				fmt.Printf("⚠️  Reply posted, but the conversation was NOT resolved.\n")
				fmt.Printf("    Finish with:  gh comment resolve %d --pr %d\n", commentID, prNumber)
				fmt.Printf("    Do not re-run review-reply — it would post the reply again.\n\n")
			}
			return fmt.Errorf("failed to resolve conversation: %w", err)
		}
		fmt.Printf("✅ Resolved conversation for review comment #%d\n", commentID)
	}

	return nil
}

// handleReviewReplyError provides intelligent error analysis and fallback suggestions for review-reply failures
func handleReviewReplyError(err error, commentID int, message, owner, repo string, prNumber int) error {
	errMsg := err.Error()

	// Check for comment type mismatch first (more specific than general 404)
	if strings.Contains(errMsg, "issues/comments") {
		fmt.Printf("\n⚠️  Comment type mismatch detected for comment #%d\n\n", commentID)
		fmt.Printf("🔍 **Issue**: This appears to be an issue comment (general PR discussion)\n")
		fmt.Printf("   • Issue comments appear in the main conversation tab\n")
		fmt.Printf("   • They don't support threading like review comments do\n\n")

		fmt.Printf("💡 **Solution**: Use the general comment command instead:\n")
		fmt.Printf("      gh comment add %d \"%s\"\n\n", prNumber, message)

		return fmt.Errorf("comment #%d is an issue comment - use 'gh comment add' for general PR discussion", commentID)
	}

	// Check for the most common issue: GitHub API limitation with review comment threading
	if strings.Contains(errMsg, "404") {
		fmt.Printf("\n⚠️  Review comment threading limitation detected for comment #%d\n\n", commentID)
		fmt.Printf("🔍 **Root Cause Analysis**:\n")
		fmt.Printf("   • GitHub's REST API only supports replies to 'standalone' review comments\n")
		fmt.Printf("   • Comments created via 'Start a Review' → 'Submit Review' cannot be replied to directly\n")
		fmt.Printf("   • This is a known GitHub API architectural limitation, not a tool issue\n\n")

		fmt.Printf("💡 **Alternative Approaches**:\n")
		fmt.Printf("   1. **Add a new comment at the same location**:\n")
		fmt.Printf("      gh comment list %d --format json | jq '.comments[] | select(.id == %d)'\n", prNumber, commentID)
		fmt.Printf("      # Find the file and line, then:\n")
		fmt.Printf("      gh comment add %d <file> <line> \"%s\"\n\n", prNumber, message)

		fmt.Printf("   2. **Use emoji reactions for quick feedback**:\n")
		fmt.Printf("      gh comment react %d +1        # Agree\n", commentID)
		fmt.Printf("      gh comment react %d heart     # Appreciate\n", commentID)
		fmt.Printf("      gh comment react %d hooray    # Celebrate\n\n", commentID)

		fmt.Printf("   3. **Try resolving the conversation** (often works even when replies don't):\n")
		fmt.Printf("      gh comment review-reply %d --resolve\n\n", commentID)

		fmt.Printf("📖 **Background**: This limitation exists because review comments are part of\n")
		fmt.Printf("    'review threads' in GitHub's data model, which have limited REST API support.\n")
		fmt.Printf("    The GraphQL API has better threading, but requires different permissions.\n\n")

		return fmt.Errorf("review comment threading not supported for this comment type - see alternatives above")
	}

	// For other errors, use the existing intelligent error system
	return fmt.Errorf("failed to create reply: %w", err)
}
