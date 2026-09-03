package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// TestClient implements GitHubAPI for testing with a configurable HTTP client
type TestClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewTestClient creates a GitHub client that can work with mock servers
func NewTestClient() (*TestClient, error) {
	// Check if we're in test mode with mock server
	mockURL := os.Getenv("MOCK_SERVER_URL")
	ghHost := os.Getenv("GH_HOST")

	var baseURL string
	if mockURL != "" {
		baseURL = mockURL
	} else if ghHost != "" && !strings.Contains(ghHost, "github.com") {
		// Custom host (like mock server)
		baseURL = "http://" + ghHost
	} else {
		baseURL = "https://api.github.com"
	}

	return &TestClient{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		token:      os.Getenv("GH_TOKEN"),
	}, nil
}

// doRequest performs an HTTP request to the GitHub API
func (c *TestClient) doRequest(method, endpoint string, body []byte) (*http.Response, error) {
	url := c.baseURL + "/" + strings.TrimPrefix(endpoint, "/")

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// ListIssueComments fetches all issue comments for a PR
func (c *TestClient) ListIssueComments(owner, repo string, prNumber int) ([]Comment, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, prNumber)

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var comments []Comment
	err = json.NewDecoder(resp.Body).Decode(&comments)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Mark as issue comments
	for i := range comments {
		comments[i].Type = "issue"
	}

	return comments, nil
}

// ListReviewComments fetches all review comments for a PR
func (c *TestClient) ListReviewComments(owner, repo string, prNumber int) ([]Comment, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, prNumber)

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var comments []Comment
	err = json.NewDecoder(resp.Body).Decode(&comments)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Mark as review comments
	for i := range comments {
		comments[i].Type = "review"
	}

	return comments, nil
}

// CreateIssueComment adds a general comment to a PR
func (c *TestClient) CreateIssueComment(owner, repo string, prNumber int, body string) (*Comment, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, prNumber)

	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := c.doRequest("POST", endpoint, jsonBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var comment Comment
	err = json.NewDecoder(resp.Body).Decode(&comment)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	comment.Type = "issue"
	return &comment, nil
}

// AddReviewComment adds a line-specific comment to a PR
func (c *TestClient) AddReviewComment(owner, repo string, pr int, comment ReviewCommentInput) error {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, pr)

	jsonBody, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment: %w", err)
	}

	resp, err := c.doRequest("POST", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

// CreateReview creates a new review with comments
func (c *TestClient) CreateReview(owner, repo string, pr int, review ReviewInput) error {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr)

	jsonBody, err := json.Marshal(review)
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}

	resp, err := c.doRequest("POST", endpoint, jsonBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetPRDetails fetches basic PR information
func (c *TestClient) GetPRDetails(owner, repo string, pr int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, pr)

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func (c *TestClient) FetchFileContent(owner, repo, path, ref string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)

	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Type     string `json:"type"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(result.Content, "\n", ""))
	if err != nil {
		return "", fmt.Errorf("failed to decode file contents: %w", err)
	}

	return string(decoded), nil
}

// Stub implementations for methods not needed in tests
func (c *TestClient) CreateReviewCommentReply(owner, repo string, commentID int, body string) (*Comment, error) {
	return nil, fmt.Errorf("not implemented in test client")
}

func (c *TestClient) FindReviewThreadForComment(owner, repo string, prNumber, commentID int) (string, error) {
	return "", fmt.Errorf("not implemented in test client")
}

func (c *TestClient) GetPRNumberForComment(owner, repo string, commentID int) (int, error) {
	return 0, fmt.Errorf("not implemented in test client")
}

func (c *TestClient) ListReviewThreads(owner, repo string, prNumber int) ([]ReviewThread, error) {
	return nil, fmt.Errorf("not implemented in test client")
}

func (c *TestClient) ResolveReviewThread(threadID string) error {
	return fmt.Errorf("not implemented in test client")
}

func (c *TestClient) AddReaction(owner, repo string, commentID int, prNumber int, reaction string) error {
	return fmt.Errorf("not implemented in test client")
}

func (c *TestClient) RemoveReaction(owner, repo string, commentID int, prNumber int, reaction string) error {
	return fmt.Errorf("not implemented in test client")
}

func (c *TestClient) EditComment(owner, repo string, commentID int, prNumber int, body string) error {
	return fmt.Errorf("not implemented in test client")
}

func (c *TestClient) FetchPRDiff(owner, repo string, pr int) (*PullRequestDiff, error) {
	return nil, fmt.Errorf("not implemented in test client")
}

func (c *TestClient) FindPendingReview(owner, repo string, pr int) (int, error) {
	return 0, fmt.Errorf("not implemented in test client")
}

func (c *TestClient) SubmitReview(owner, repo string, pr, reviewID int, body, event string) error {
	return fmt.Errorf("not implemented in test client")
}
