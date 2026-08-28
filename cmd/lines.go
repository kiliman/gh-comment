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
	showCodeContext bool
	linesClient     github.GitHubAPI
)

var linesCmd = &cobra.Command{
	Use:   "lines <pr> <file>",
	Short: "Show commentable lines in a PR file",
	Long: heredoc.Doc(`
		Show which lines in a file can receive comments based on the PR diff.

		This command helps debug failed comment attempts by showing exactly which
		lines are available for commenting. GitHub only allows comments on lines
		that are part of the diff (added, modified, or in context).

		Note: Newly added files may not show commentable lines due to GitHub API
		limitations. If no lines are shown, try commenting on any line directly.

		Use this command when you get HTTP 422 errors trying to add line comments.
	`),
	Example: heredoc.Doc(`
		# Show commentable lines in a specific file
		$ gh comment lines 123 src/main.go

		# Show lines with code context
		$ gh comment lines 123 src/main.go --show-code

		# Check if specific line is commentable
		$ gh comment lines 123 src/main.go | grep "^42:"

		# Get line ranges for scripting
		$ gh comment lines 123 src/main.go | grep -o "^[0-9]*" | head -5
	`),
	Args: cobra.ExactArgs(2),
	RunE: runLines,
}

func init() {
	rootCmd.AddCommand(linesCmd)
	linesCmd.Flags().BoolVar(&showCodeContext, "show-code", true, "Show actual code content for each line")
}

func runLines(cmd *cobra.Command, args []string) error {
	// Initialize client if not set (production use)
	if linesClient == nil {
		client, err := createGitHubClient()
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}
		linesClient = client
	}

	// Parse PR number
	pr, err := strconv.Atoi(args[0])
	if err != nil {
		return formatValidationError("PR number", args[0], "must be a valid integer")
	}

	// Get file path
	filePath := args[1]

	// Get repository context
	repository, err := getCurrentRepo()
	if err != nil {
		return err
	}

	// Parse owner/repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repoName := parts[0], parts[1]

	if verbose {
		fmt.Printf("Repository: %s\n", repository)
		fmt.Printf("PR: %d\n", pr)
		fmt.Printf("File: %s\n", filePath)
		fmt.Println()
	}

	if dryRun {
		fmt.Printf("Would show commentable lines for file %s in PR #%d\n", filePath, pr)
		return nil
	}

	// Fetch PR diff
	diff, err := linesClient.FetchPRDiff(owner, repoName, pr)
	if err != nil {
		return fmt.Errorf("failed to fetch PR diff: %w", err)
	}

	// Find the requested file in the diff
	var targetFile *github.DiffFile
	for i := range diff.Files {
		if diff.Files[i].Filename == filePath {
			targetFile = &diff.Files[i]
			break
		}
	}

	if targetFile == nil {
		fmt.Printf("❌ File '%s' not found in PR #%d diff\n\n", filePath, pr)
		fmt.Println("Available files in this PR:")
		for _, file := range diff.Files {
			fmt.Printf("  • %s\n", file.Filename)
		}
		return nil
	}

	// Display commentable lines
	if len(targetFile.Lines) == 0 {
		fmt.Printf("❌ No commentable lines found in %s\n", filePath)
		fmt.Println("This file may not have any changes in this PR.")
		return nil
	}

	fmt.Printf("✅ Commentable lines in %s (PR #%d):\n\n", filePath, pr)

	// Sort line numbers
	var lineNumbers []int
	for lineNum := range targetFile.Lines {
		lineNumbers = append(lineNumbers, lineNum)
	}
	sort.Ints(lineNumbers)

	// Group consecutive lines for better display
	ranges := groupConsecutiveLines(lineNumbers)

	var fileLines []string
	if showCodeContext {
		fileLines, err = fetchPRFileLines(linesClient, owner, repoName, pr, filePath)
		if err != nil {
			return fmt.Errorf("failed to fetch file contents for %s: %w", filePath, err)
		}
	}

	fmt.Printf("📍 Line ranges available for comments:\n")
	for _, lineRange := range ranges {
		if lineRange.start == lineRange.end {
			fmt.Printf("  • Line %d\n", lineRange.start)
		} else {
			fmt.Printf("  • Lines %d-%d\n", lineRange.start, lineRange.end)
		}
	}

	fmt.Printf("\n📝 Individual lines:\n")
	for _, lineNum := range lineNumbers {
		if showCodeContext {
			fmt.Printf("%d: %s\n", lineNum, lineContentAt(fileLines, lineNum))
		} else {
			fmt.Printf("%d\n", lineNum)
		}
	}

	fmt.Printf("\n💡 Usage examples:\n")
	fmt.Printf("  • Single line:  gh comment add %d %s %d \"Your comment\"\n", pr, filePath, lineNumbers[0])
	if len(lineNumbers) > 1 {
		fmt.Printf("  • Range comment: gh comment add %d %s %d:%d \"Range comment\"\n", pr, filePath, lineNumbers[0], lineNumbers[len(lineNumbers)-1])
	}

	return nil
}

func fetchPRFileLines(client github.GitHubAPI, owner, repoName string, pr int, filePath string) ([]string, error) {
	prDetails, err := client.GetPRDetails(owner, repoName, pr)
	if err != nil {
		return nil, err
	}

	head, ok := prDetails["head"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("PR #%d is missing head commit details", pr)
	}

	sha, ok := head["sha"].(string)
	if !ok || strings.TrimSpace(sha) == "" {
		return nil, fmt.Errorf("PR #%d is missing head SHA", pr)
	}

	content, err := client.FetchFileContent(owner, repoName, filePath, sha)
	if err != nil {
		return nil, err
	}

	return strings.Split(content, "\n"), nil
}

func lineContentAt(lines []string, lineNum int) string {
	if lineNum <= 0 || lineNum > len(lines) {
		return "[line content unavailable]"
	}
	return lines[lineNum-1]
}
