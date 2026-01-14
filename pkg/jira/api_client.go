package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"thoreinstein.com/rig/pkg/config"
)

// Rate limit retry configuration
const (
	maxRetries   = 3
	baseDelay    = time.Second
	maxDelay     = 30 * time.Second
	jitterFactor = 0.4 // jitterFactor = 0.4 produces a multiplier range of [0.8, 1.2], which is ±20% variation around 1.0
)

// Compile-time interface check
var _ JiraClient = (*APIClient)(nil)

// APIClient implements JiraClient using Jira Cloud REST API v3
type APIClient struct {
	baseURL      string
	email        string
	token        string
	customFields map[string]string
	httpClient   *http.Client
	verbose      bool
}

// NewAPIClient creates a new API-based Jira client.
// Token lookup precedence: JIRA_TOKEN env var > config token.
func NewAPIClient(cfg *config.JiraConfig, verbose bool) (*APIClient, error) {
	// Token from env var takes precedence
	token := os.Getenv("JIRA_TOKEN")
	if token == "" {
		token = cfg.Token
	}

	if cfg.BaseURL == "" {
		return nil, errors.New("jira base_url is required for API mode")
	}
	if cfg.Email == "" {
		return nil, errors.New("jira email is required for API mode")
	}
	if token == "" {
		return nil, errors.New("jira token is required (set JIRA_TOKEN env var or config)")
	}

	return &APIClient{
		baseURL:      strings.TrimSuffix(cfg.BaseURL, "/"),
		email:        cfg.Email,
		token:        token,
		customFields: cfg.CustomFields,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		verbose:      verbose,
	}, nil
}

// IsAvailable checks if the API client is configured and ready to use.
func (c *APIClient) IsAvailable() bool {
	return c.baseURL != "" && c.email != "" && c.token != ""
}

// calculateBackoff computes the delay for a retry attempt using exponential backoff with jitter.
// Formula: delay = min(initial * 2^attempt, max) * (0.8 + 0.4*rand())
func calculateBackoff(base, max time.Duration, attempt int) time.Duration {
	// Calculate exponential delay: base * 2^attempt
	expDelay := float64(base) * math.Pow(2, float64(attempt))

	// Cap at max delay
	if expDelay > float64(max) {
		expDelay = float64(max)
	}

	// Apply jitter: multiply by (0.8 + 0.4*rand()) which gives range [0.8, 1.2]
	// This is ±20% variation around the base value
	jitter := 0.8 + jitterFactor*rand.Float64()
	delay := time.Duration(expDelay * jitter)

	return delay
}

// parseRetryAfter extracts the delay from a Retry-After header.
// Returns the duration if present and valid, otherwise returns 0.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date (RFC1123)
	if t, err := time.Parse(time.RFC1123, header); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}

	return 0
}

// doRequestWithRetry executes an HTTP request with retry logic for rate limiting.
// It implements exponential backoff with jitter and respects Retry-After headers.
func (c *APIClient) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone the request for retry (body has already been read)
		// Since we're doing GET requests, we don't need to worry about body cloning
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to execute request")
		}

		// If not rate limited, return the response
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// Close the response body before retry
		resp.Body.Close()

		// Check if we've exhausted retries
		if attempt == maxRetries {
			lastErr = errors.Newf("rate limited after %d retries", maxRetries)
			break
		}

		// Calculate delay, preferring Retry-After header if present
		delay := parseRetryAfter(resp.Header.Get("Retry-After"))
		if delay == 0 {
			delay = calculateBackoff(baseDelay, maxDelay, attempt)
		}

		if c.verbose {
			fmt.Printf("Rate limited (HTTP 429), retrying in %v (attempt %d/%d)...\n",
				delay.Round(time.Millisecond), attempt+1, maxRetries)
		}

		time.Sleep(delay)
	}

	return nil, lastErr
}

// FetchTicketDetails retrieves ticket information from Jira using the REST API v3.
func (c *APIClient) FetchTicketDetails(ticket string) (*TicketInfo, error) {
	if !c.IsAvailable() {
		return nil, errors.New("jira API client is not configured")
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, ticket)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Set Basic Auth header: base64(email:token)
	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	if c.verbose {
		fmt.Printf("Fetching Jira ticket: %s\n", url)
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(resp.StatusCode, body, ticket)
	}

	return c.parseResponse(body)
}

// handleHTTPError returns an appropriate error for non-200 responses.
// Note: HTTP 429 (rate limit) is handled by doRequestWithRetry before this is called.
func (c *APIClient) handleHTTPError(statusCode int, body []byte, ticket string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return errors.Newf("authentication failed: check your email and API token (HTTP 401)")
	case http.StatusForbidden:
		return errors.Newf("access denied to ticket %s: check your permissions (HTTP 403)", ticket)
	case http.StatusNotFound:
		return errors.Newf("ticket %s not found (HTTP 404)", ticket)
	case http.StatusTooManyRequests:
		// This case should not be reached in normal flow as doRequestWithRetry handles 429s
		return errors.New("rate limit exceeded after retries: please wait before making more requests (HTTP 429)")
	default:
		// Try to extract error message from response
		var errResp struct {
			ErrorMessages []string `json:"errorMessages"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.ErrorMessages) > 0 {
			return errors.Newf("jira API error (HTTP %d): %s", statusCode, strings.Join(errResp.ErrorMessages, "; "))
		}
		return errors.Newf("jira API error (HTTP %d)", statusCode)
	}
}

// jiraIssueResponse represents the relevant parts of a Jira API v3 issue response.
type jiraIssueResponse struct {
	Fields struct {
		IssueType   *jiraNameField   `json:"issuetype"`
		Summary     string           `json:"summary"`
		Status      *jiraNameField   `json:"status"`
		Priority    *jiraNameField   `json:"priority"`
		Description *jiraADFDocument `json:"description"`
	} `json:"fields"`
}

// jiraNameField represents a Jira field with a name property.
type jiraNameField struct {
	Name string `json:"name"`
}

// jiraADFDocument represents an Atlassian Document Format document.
// ADF is a nested JSON structure used by Jira Cloud API v3 for rich text fields.
type jiraADFDocument struct {
	Type    string           `json:"type"`
	Content []jiraADFContent `json:"content"`
}

// jiraADFContent represents a content node in an ADF document.
type jiraADFContent struct {
	Type    string           `json:"type"`
	Text    string           `json:"text,omitempty"`
	Content []jiraADFContent `json:"content,omitempty"`
}

// parseResponse parses the Jira API response and extracts ticket information.
func (c *APIClient) parseResponse(body []byte) (*TicketInfo, error) {
	var resp jiraIssueResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "failed to parse jira response")
	}

	info := &TicketInfo{
		Summary: resp.Fields.Summary,
	}

	if resp.Fields.IssueType != nil {
		info.Type = resp.Fields.IssueType.Name
	}
	if resp.Fields.Status != nil {
		info.Status = resp.Fields.Status.Name
	}
	if resp.Fields.Priority != nil {
		info.Priority = resp.Fields.Priority.Name
	}
	if resp.Fields.Description != nil {
		info.Description = extractADFText(resp.Fields.Description)
	}

	// Extract custom fields if configured
	if len(c.customFields) > 0 {
		info.CustomFields = c.extractCustomFields(body)
	}

	if c.verbose {
		fmt.Printf("Fetched Jira details for ticket: %s\n", info.Summary)
	}

	return info, nil
}

// extractCustomFields extracts custom field values from the raw JSON response.
// It uses the configured mapping of friendly names to Jira field IDs.
func (c *APIClient) extractCustomFields(body []byte) map[string]string {
	// Parse the response to get raw fields
	var raw struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	result := make(map[string]string)
	for friendlyName, fieldID := range c.customFields {
		rawValue, ok := raw.Fields[fieldID]
		if !ok || len(rawValue) == 0 || string(rawValue) == "null" {
			continue
		}

		value := extractCustomFieldValue(rawValue)
		if value != "" {
			result[friendlyName] = value
		}
	}

	return result
}

// extractCustomFieldValue converts a raw JSON custom field value to a string.
// Handles different value types: string, number, object with value/name, array.
func extractCustomFieldValue(raw json.RawMessage) string {
	// Try string first
	var strVal string
	if err := json.Unmarshal(raw, &strVal); err == nil {
		return strVal
	}

	// Try number (float64 handles both int and float)
	var numVal float64
	if err := json.Unmarshal(raw, &numVal); err == nil {
		// Use strconv for clean integer formatting when possible
		if numVal == float64(int64(numVal)) {
			return strconv.FormatInt(int64(numVal), 10)
		}
		return strconv.FormatFloat(numVal, 'f', -1, 64)
	}

	// Try object with value or name field
	var objVal struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objVal); err == nil {
		if objVal.Value != "" {
			return objVal.Value
		}
		if objVal.Name != "" {
			return objVal.Name
		}
	}

	// Try array of objects with value or name
	var arrVal []struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &arrVal); err == nil && len(arrVal) > 0 {
		var values []string
		for _, item := range arrVal {
			if item.Value != "" {
				values = append(values, item.Value)
			} else if item.Name != "" {
				values = append(values, item.Name)
			}
		}
		if len(values) > 0 {
			return strings.Join(values, ", ")
		}
	}

	// Try array of strings
	var strArr []string
	if err := json.Unmarshal(raw, &strArr); err == nil && len(strArr) > 0 {
		return strings.Join(strArr, ", ")
	}

	return ""
}

// extractADFText extracts plain text from an Atlassian Document Format document.
// ADF is a tree structure where text is found in leaf nodes of type "text".
func extractADFText(doc *jiraADFDocument) string {
	if doc == nil {
		return ""
	}

	var parts []string
	for _, content := range doc.Content {
		text := extractADFContentText(&content)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// extractADFContentText recursively extracts text from an ADF content node.
func extractADFContentText(content *jiraADFContent) string {
	if content == nil {
		return ""
	}

	// If this is a text node, return its text
	if content.Type == "text" {
		return content.Text
	}

	// Otherwise, recursively extract text from children
	var parts []string
	for _, child := range content.Content {
		text := extractADFContentText(&child)
		if text != "" {
			parts = append(parts, text)
		}
	}

	// Join based on content type
	switch content.Type {
	case "paragraph", "heading", "listItem":
		return strings.Join(parts, "")
	case "bulletList", "orderedList":
		return strings.Join(parts, "\n")
	default:
		return strings.Join(parts, "")
	}
}

// jiraTransitionsResponse represents the response from the transitions endpoint.
type jiraTransitionsResponse struct {
	Transitions []jiraTransition `json:"transitions"`
}

// jiraTransitionStatus represents a status in the transition response.
type jiraTransitionStatus struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// jiraTransition represents a single transition in the API response.
type jiraTransition struct {
	ID   string               `json:"id"`
	Name string               `json:"name"`
	To   jiraTransitionStatus `json:"to"`
}

// GetTransitions returns the available workflow transitions for a ticket.
// GET /rest/api/3/issue/{issueKey}/transitions
func (c *APIClient) GetTransitions(ticket string) ([]Transition, error) {
	if !c.IsAvailable() {
		return nil, errors.New("jira API client is not configured")
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, ticket)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	if c.verbose {
		fmt.Printf("Fetching transitions for ticket: %s\n", ticket)
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleHTTPError(resp.StatusCode, body, ticket)
	}

	var transResp jiraTransitionsResponse
	if err := json.Unmarshal(body, &transResp); err != nil {
		return nil, errors.Wrap(err, "failed to parse transitions response")
	}

	transitions := make([]Transition, len(transResp.Transitions))
	for i, t := range transResp.Transitions {
		transitions[i] = Transition{
			ID:   t.ID,
			Name: t.Name,
			To: TransitionStatus{
				ID:   t.To.ID,
				Name: t.To.Name,
			},
		}
	}

	if c.verbose {
		fmt.Printf("Found %d available transitions for %s\n", len(transitions), ticket)
	}

	return transitions, nil
}

// jiraTransitionRequest represents the request body for executing a transition.
type jiraTransitionRequest struct {
	Transition jiraTransitionID `json:"transition"`
}

// jiraTransitionID represents the transition ID in the request body.
type jiraTransitionID struct {
	ID string `json:"id"`
}

// TransitionTicket executes a workflow transition by its ID.
// POST /rest/api/3/issue/{issueKey}/transitions
// Body: {"transition": {"id": "31"}}
func (c *APIClient) TransitionTicket(ticket string, transitionID string) error {
	if !c.IsAvailable() {
		return errors.New("jira API client is not configured")
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, ticket)

	reqBody := jiraTransitionRequest{
		Transition: jiraTransitionID{ID: transitionID},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request body")
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.verbose {
		fmt.Printf("Transitioning ticket %s with transition ID %s\n", ticket, transitionID)
	}

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 204 No Content is the success response for transitions
	if resp.StatusCode == http.StatusNoContent {
		if c.verbose {
			fmt.Printf("Successfully transitioned ticket %s\n", ticket)
		}
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	return c.handleTransitionError(resp.StatusCode, body, ticket, transitionID)
}

// handleTransitionError returns an appropriate error for transition failures.
func (c *APIClient) handleTransitionError(statusCode int, body []byte, ticket, transitionID string) error {
	switch statusCode {
	case http.StatusBadRequest:
		// Try to extract error details from response
		var errResp struct {
			ErrorMessages []string          `json:"errorMessages"`
			Errors        map[string]string `json:"errors"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if len(errResp.ErrorMessages) > 0 {
				return errors.Newf("invalid transition %s for ticket %s: %s", transitionID, ticket, strings.Join(errResp.ErrorMessages, "; "))
			}
			if len(errResp.Errors) > 0 {
				var errs []string
				for field, msg := range errResp.Errors {
					errs = append(errs, fmt.Sprintf("%s: %s", field, msg))
				}
				return errors.Newf("invalid transition %s for ticket %s: %s", transitionID, ticket, strings.Join(errs, "; "))
			}
		}
		return errors.Newf("invalid transition %s for ticket %s (HTTP 400)", transitionID, ticket)
	case http.StatusNotFound:
		return errors.Newf("ticket %s not found (HTTP 404)", ticket)
	default:
		return c.handleHTTPError(statusCode, body, ticket)
	}
}

// TransitionTicketByName finds a transition by status name and executes it.
// It performs a case-insensitive match on the transition name or target status name.
func (c *APIClient) TransitionTicketByName(ticket string, statusName string) error {
	transitions, err := c.GetTransitions(ticket)
	if err != nil {
		return errors.Wrap(err, "failed to get available transitions")
	}

	statusLower := strings.ToLower(statusName)
	var matchedTransition *Transition

	for i := range transitions {
		t := &transitions[i]
		// Match on transition name or target status name (case-insensitive)
		if strings.ToLower(t.Name) == statusLower || strings.ToLower(t.To.Name) == statusLower {
			matchedTransition = t
			break
		}
	}

	if matchedTransition == nil {
		available := make([]string, len(transitions))
		for i, t := range transitions {
			available[i] = fmt.Sprintf("%s -> %s", t.Name, t.To.Name)
		}
		return errors.Newf("transition to %q not found for ticket %s; available: [%s]",
			statusName, ticket, strings.Join(available, ", "))
	}

	if c.verbose {
		fmt.Printf("Found transition %q (ID: %s) -> %s for ticket %s\n",
			matchedTransition.Name, matchedTransition.ID, matchedTransition.To.Name, ticket)
	}

	return c.TransitionTicket(ticket, matchedTransition.ID)
}
