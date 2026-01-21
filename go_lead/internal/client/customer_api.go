package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/checkfox/go_lead/internal/models"
)

// CustomerAPIClient handles communication with the external Customer API
type CustomerAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewCustomerAPIClient creates a new Customer API client
func NewCustomerAPIClient(baseURL, token string, timeout time.Duration) *CustomerAPIClient {
	return &CustomerAPIClient{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// DeliveryResponse represents the response from the Customer API
type DeliveryResponse struct {
	StatusCode   int
	Body         string
	Success      bool
	ErrorMessage string
}

// SendLead sends a lead to the Customer API
// Returns DeliveryResponse and error
// The error will be a *models.DeliveryError with Retriable flag set appropriately
func (c *CustomerAPIClient) SendLead(ctx context.Context, payload map[string]interface{}) (*DeliveryResponse, error) {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, models.NewDeliveryError(0, "failed to marshal payload", false, err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, models.NewDeliveryError(0, "failed to create request", false, err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network errors are retriable
		return nil, models.NewDeliveryError(0, "network error", true, err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// Failed to read response body - treat as retriable
		return nil, models.NewDeliveryError(resp.StatusCode, "failed to read response body", true, err)
	}

	bodyString := string(bodyBytes)

	// Determine if the response indicates success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &DeliveryResponse{
			StatusCode: resp.StatusCode,
			Body:       bodyString,
			Success:    true,
		}, nil
	}

	// Handle error responses
	retriable := isRetriableStatusCode(resp.StatusCode)
	errorMessage := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bodyString)

	return &DeliveryResponse{
		StatusCode:   resp.StatusCode,
		Body:         bodyString,
		Success:      false,
		ErrorMessage: errorMessage,
	}, models.NewDeliveryError(resp.StatusCode, errorMessage, retriable, nil)
}

// isRetriableStatusCode determines if an HTTP status code should trigger a retry
func isRetriableStatusCode(statusCode int) bool {
	// 5xx errors are retriable (server errors)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// 429 Too Many Requests is retriable
	if statusCode == 429 {
		return true
	}

	// 4xx errors (except 429) are not retriable (client errors)
	if statusCode >= 400 && statusCode < 500 {
		return false
	}

	// Other status codes (3xx, etc.) are not retriable
	return false
}
