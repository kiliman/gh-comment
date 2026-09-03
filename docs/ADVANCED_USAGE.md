# Advanced Usage Guide

Power user features and automation workflows for gh-comment.

## Advanced Command Usage

### `gh comment list` - Advanced Filtering

The list command supports sophisticated filtering options:

```bash
# Filter by author with wildcards
gh comment list 123 --author "senior-dev*"     # Users starting with "senior-dev"
gh comment list 123 --author "*@company.com"   # Organization members
gh comment list 123 --author "!*[bot]"         # Exclude bots

# Date range filtering
gh comment list 123 --since "1 week ago" --until "yesterday"
gh comment list 123 --since "2024-01-01" --until "2024-03-31"

# Combined filtering for analysis
gh comment list 123 --type review --status open --author "team-lead*" --since "1 week ago"
```

**Use cases:**
- Review process analysis
- Finding unresolved feedback
- Tracking specific reviewer patterns

### `gh comment batch` - Complex Workflows

Advanced batch configurations for systematic reviews:

```yaml
# security-audit.yaml
pr: 123
review:
  body: "Security audit complete - critical issues found"
  event: "REQUEST_CHANGES"
comments:
  - file: src/api.js
    line: 42
    message: "🔒 Use crypto.randomBytes(32) instead of Math.random()"
  - file: src/auth.js
    line: 15
    message: "🛡️ Add rate limiting middleware (express-rate-limit)"
  - type: issue
    message: "Please address all security findings before merge"
```

```bash
# Process with validation
gh comment batch 123 security-audit.yaml --dry-run --verbose
gh comment batch 123 security-audit.yaml
```

### `gh comment review` - Multi-Comment Reviews

Create comprehensive reviews in one command:

```bash
# Architecture review with multiple findings
gh comment review 123 "Architecture review complete" \
  --comment src/api.js:42:"Extract this logic to a service layer" \
  --comment src/database.js:15:25:"Consider using repository pattern here" \
  --comment src/auth.js:67:"Move authentication logic to middleware" \
  --event REQUEST_CHANGES

# Performance review with suggestions
gh comment review 123 "Performance optimization opportunities" \
  --comment components/Table.js:124:"Consider virtualizing for large datasets" \
  --comment utils/api.js:67:"Add request caching for expensive operations" \
  --event COMMENT
```

### `gh comment close-pending-review` - GitHub Review Limitation Workaround

**The Problem:** GitHub's API has a critical limitation - you cannot create a new review if you already have a pending (draft) review open on the same PR. This commonly happens when:

1. You start a review in GitHub's web interface but don't submit it
2. You use the "Start a review" button and add comments but leave it as draft
3. A previous review got stuck in pending state due to an error

**The Solution:** Use `close-pending-review` to submit your pending review, then you can create new reviews.

```bash
# Check if you have a pending review blocking new reviews
gh comment review 123 "New review" --comment src/api.js:42:"Test comment"
# ❌ Error: cannot create review - pending review exists

# Submit the pending review first
gh comment close-pending-review 123 "Submitting previous draft review" --event COMMENT

# Now you can create new reviews
gh comment review 123 "New review works!" --comment src/api.js:42:"Test comment"
# ✅ Success
```

**Common Workflow:**
```bash
# 1. Start review in GitHub web UI (accidentally leaves it pending)
# 2. Try to use CLI - gets blocked
gh comment review 123 "Security review" --comment src/auth.js:15:"Use bcrypt"
# Error: Pending review exists

# 3. Submit pending review
gh comment close-pending-review 123 "Completing web UI review" --event APPROVE

# 4. Now CLI reviews work normally  
gh comment review 123 "Follow-up security review" --comment src/api.js:42:"Add rate limiting"
```

**Events you can use:**
- `COMMENT` - General feedback (default)
- `APPROVE` - Approve the PR
- `REQUEST_CHANGES` - Block merge until issues addressed

### `gh comment add` - Suggestion Syntax

Advanced suggestion patterns:

```bash
# Simple inline suggestions
gh comment add 123 "Try this approach: [SUGGEST: const result = data?.filter(x => x.active) || []]"

# Multi-line suggestions
gh comment add 123 "Consider this refactor:

<<<SUGGEST
function processUsers(users) {
  return users
    .filter(user => user.active)
    .map(user => normalizeUser(user));
}
SUGGEST>>>

This separates concerns better."

# Offset suggestions for context
gh comment add 123 src/api.js 42 "Add validation: [SUGGEST:-1: if (!input) throw new Error('Input required');]"
gh comment add 123 src/api.js 42 "Add logging: [SUGGEST:+1: logger.info('Request processed', { id });]"
```

## Automation Workflows

### Systematic Code Review Script

```bash
#!/bin/bash
# comprehensive-review.sh

PR=$1
echo "🔍 Starting systematic review of PR #$PR"

# 1. Analyze structure
echo "📁 Files changed:"
gh pr diff $PR --name-only

# 2. Security review
if gh pr diff $PR --name-only | grep -E "(auth|security|api)" > /dev/null; then
  echo "🔒 Running security review..."
  gh comment batch $PR security-template.yaml
fi  

# 3. Performance check
if gh pr diff $PR --name-only | grep -E "(component|util|service)" > /dev/null; then
  echo "⚡ Checking performance patterns..."
  gh comment review $PR "Performance review" \
    --comment src/components/DataTable.js:124:"Consider virtualization for large lists" \
    --event COMMENT
fi

# 4. Generate summary
echo "📊 Review complete:"
gh comment list $PR --since "1 hour ago" --quiet | wc -l | xargs echo "New comments:"
```

### CI/CD Integration

gh-comment works seamlessly in CI/CD pipelines. Install the extension and use it directly:

```bash
# Install in CI
gh extension install silouanwright/gh-comment

# Use in scripts
gh comment review $PR_NUMBER "Automated security scan results" \
  --comment src/api.js:42:"Vulnerability detected: use crypto.randomBytes()" \
  --event REQUEST_CHANGES
```

### Automated Comment Management

```bash
# cleanup-resolved.sh - Mark resolved conversations
PR=$1

echo "🧹 Cleaning up resolved conversations on PR #$PR"

# Get resolved review comment IDs
RESOLVED_IDS=$(gh comment list $PR --type review --status resolved --quiet | \
  grep "ID:" | cut -d':' -f2)

# Add celebration reactions to resolved comments
for id in $RESOLVED_IDS; do
  gh comment react $id hooray --pr $PR
  echo "✅ Celebrated resolved comment $id"
done
```

## Configuration Management

### Project-Specific Configuration

Create `.gh-comment.yaml` in your project root:

```yaml
# .gh-comment.yaml
defaults:
  repository: "myorg/myproject"

behavior:
  validate: true      # Always validate line numbers
  verbose: false      # Quiet by default
  dry_run: false

review:
  event: "COMMENT"    # Default review event

templates:
  security_review: |
    🔒 Security Review Checklist:
    - [ ] Input validation
    - [ ] Authentication checks  
    - [ ] Authorization logic
    - [ ] Error handling
```

```bash
# Commands will use project configuration automatically
gh comment config show                    # View effective config
gh comment config validate .gh-comment.yaml    # Validate config file
```

## Data Export and Analysis

### Comment Analytics

```bash
# Export for analysis  
gh comment export 123 > pr-123-analysis.json

# Generate statistics
echo "📊 PR Comment Statistics:"
echo "Total comments: $(gh comment list 123 --quiet | wc -l)"
echo "Review comments: $(gh comment list 123 --type review --quiet | wc -l)"  
echo "Issue comments: $(gh comment list 123 --type issue --quiet | wc -l)"

# Top commenters
echo "👥 Most Active Reviewers:"
gh comment list 123 --quiet | grep "👤" | cut -d' ' -f2 | sort | uniq -c | sort -nr | head -5
```

### Integration with External Tools

```bash
# JIRA integration - link related tickets
ISSUE_KEY=$(gh pr view $PR --json title | jq -r '.title' | grep -o '[A-Z]+-[0-9]+')
if [ -n "$ISSUE_KEY" ]; then
  gh comment add $PR "🔗 Related JIRA: https://company.atlassian.net/browse/$ISSUE_KEY"
fi

# Custom notifications
gh comment list $PR --since "1 hour ago" --quiet | \
while IFS= read -r comment; do
  # Process new comments for your notification system
  echo "New comment: $comment" >> notifications.log
done
```

## Command Combinations

### Review Process Workflows

```bash
# 1. Initial review submission
gh comment review 123 "Initial code review" \
  --comment src/api.js:42:"Add error handling" \
  --comment src/auth.js:15:"Use bcrypt for passwords" \
  --event REQUEST_CHANGES

# 2. Follow up on specific issues  
gh comment review-reply 2254752948 "Thanks for the fix! Please also add unit tests."

# 3. Approve after changes
gh comment review 123 "All issues addressed - approved!" --event APPROVE

# 4. Track resolution
gh comment list 123 --type review
```

### Bulk Operations

```bash
# React to multiple comments
gh comment list 123 --author "junior-dev" --quiet | \
  grep "ID:" | cut -d':' -f2 | \
  xargs -I {} gh comment react {} +1 --pr 123

# Batch edit old comments
gh comment list 123 --until "1 week ago" --quiet | \
  grep "ID:" | head -5 | cut -d':' -f2 | \
  while read id; do
    gh comment edit $id "Updated: $original_message" --pr 123
  done
```

## Troubleshooting Advanced Scenarios

### Common Complex Issues

**Batch operations failing with line validation errors:**
```bash
# First, check which lines are commentable
gh comment lines 123 src/api.js

# Then update your batch file to use valid lines
# Or disable validation for the batch
gh comment batch 123 review.yaml --validate=false
```

**Large PR performance:**
```bash  
# Use filtering to reduce data transfer
gh comment list 123 --type review --status open --since "1 week ago"

# Export and process locally for analysis
gh comment export 123 | jq '.[] | select(.type == "review")'
```

**API rate limiting:**
```bash
# Add delays between batch operations
sleep 1 && gh comment batch 123 part1.yaml
sleep 1 && gh comment batch 123 part2.yaml

# Check rate limit status
gh api rate_limit
```

## Best Practices for Advanced Usage

1. **Always test with --dry-run** for complex operations
2. **Use --verbose** to understand API interactions
3. **Combine related comments** into single reviews  
4. **Filter aggressively** to reduce API calls
5. **Cache results** with export for repeated analysis
6. **Script repetitive tasks** for consistency
7. **Use configuration files** for team standards

For more basic usage, see the main [README](../README.md) or run `gh comment --help`.