package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkfox/go_lead/internal/models"
)

func TestSendLead_Success(t *testing.T) {
	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify Content-Type header
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		// Verify Authorization header
		expectedAuth := "Bearer test-token-123"
		if auth := r.Header.Get("Authorization"); auth != expectedAuth {
			t.Errorf("Expected Authorization %s, got %s", expectedAuth, auth)
		}

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": "lead-123", "status": "accepted"}`))
	}))
	defer server.Close()

	// Create client
	client := NewCustomerAPIClient(server.URL, "test-token-123", 30*time.Second)

	// Send lead
	payload := map[string]interface{}{
		"phone":   "1234567890",
		"email":   "test@example.com",
		"product": map[string]interface{}{"name": "Solar Panels"},
	}

	resp, err := client.SendLead(context.Background(), payload)

	// Verify no error
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify response
	if !resp.Success {
		t.Errorf("Expected success=true, got false")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	if resp.Body == "" {
		t.Errorf("Expected non-empty response body")
	}
}

func TestSendLead_2xxResponses(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"202 Accepted", http.StatusAccepted},
		{"204 No Content", http.StatusNoContent},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(`{"status": "ok"}`))
			}))
			defer server.Close()

			client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)
			payload := map[string]interface{}{"phone": "1234567890"}

			resp, err := client.SendLead(context.Background(), payload)

			if err != nil {
				t.Fatalf("Expected no error for %d response, got %v", tc.statusCode, err)
			}

			if !resp.Success {
				t.Errorf("Expected success=true for %d response", tc.statusCode)
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestSendLead_NonRetriable4xxErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"422 Unprocessable Entity", http.StatusUnprocessableEntity},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(`{"error": "client error"}`))
			}))
			defer server.Close()

			client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)
			payload := map[string]interface{}{"phone": "1234567890"}

			resp, err := client.SendLead(context.Background(), payload)

			// Should return an error
			if err == nil {
				t.Fatalf("Expected error for %d response, got nil", tc.statusCode)
			}

			// Error should be a DeliveryError
			deliveryErr, ok := err.(*models.DeliveryError)
			if !ok {
				t.Fatalf("Expected *models.DeliveryError, got %T", err)
			}

			// Should NOT be retriable
			if deliveryErr.IsRetriable() {
				t.Errorf("Expected non-retriable error for %d response", tc.statusCode)
			}

			// Response should indicate failure
			if resp.Success {
				t.Errorf("Expected success=false for %d response", tc.statusCode)
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestSendLead_Retriable5xxErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
		{"504 Gateway Timeout", http.StatusGatewayTimeout},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(`{"error": "server error"}`))
			}))
			defer server.Close()

			client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)
			payload := map[string]interface{}{"phone": "1234567890"}

			resp, err := client.SendLead(context.Background(), payload)

			// Should return an error
			if err == nil {
				t.Fatalf("Expected error for %d response, got nil", tc.statusCode)
			}

			// Error should be a DeliveryError
			deliveryErr, ok := err.(*models.DeliveryError)
			if !ok {
				t.Fatalf("Expected *models.DeliveryError, got %T", err)
			}

			// Should be retriable
			if !deliveryErr.IsRetriable() {
				t.Errorf("Expected retriable error for %d response", tc.statusCode)
			}

			// Response should indicate failure
			if resp.Success {
				t.Errorf("Expected success=false for %d response", tc.statusCode)
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("Expected status code %d, got %d", tc.statusCode, resp.StatusCode)
			}
		})
	}
}

func TestSendLead_429TooManyRequests_Retriable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limit exceeded"}`))
	}))
	defer server.Close()

	client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)
	payload := map[string]interface{}{"phone": "1234567890"}

	resp, err := client.SendLead(context.Background(), payload)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error for 429 response, got nil")
	}

	// Error should be a DeliveryError
	deliveryErr, ok := err.(*models.DeliveryError)
	if !ok {
		t.Fatalf("Expected *models.DeliveryError, got %T", err)
	}

	// 429 should be retriable
	if !deliveryErr.IsRetriable() {
		t.Error("Expected 429 Too Many Requests to be retriable")
	}

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status code 429, got %d", resp.StatusCode)
	}
}

func TestSendLead_NetworkError(t *testing.T) {
	// Use an invalid URL to trigger a network error
	client := NewCustomerAPIClient("http://invalid-host-that-does-not-exist-12345.com", "token", 1*time.Second)
	payload := map[string]interface{}{"phone": "1234567890"}

	_, err := client.SendLead(context.Background(), payload)

	// Should return an error
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}

	// Error should be a DeliveryError
	deliveryErr, ok := err.(*models.DeliveryError)
	if !ok {
		t.Fatalf("Expected *models.DeliveryError, got %T", err)
	}

	// Network errors should be retriable
	if !deliveryErr.IsRetriable() {
		t.Error("Expected network error to be retriable")
	}
}

func TestSendLead_Timeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	client := NewCustomerAPIClient(server.URL, "token", 100*time.Millisecond)
	payload := map[string]interface{}{"phone": "1234567890"}

	_, err := client.SendLead(context.Background(), payload)

	// Should return an error
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Error should be a DeliveryError
	deliveryErr, ok := err.(*models.DeliveryError)
	if !ok {
		t.Fatalf("Expected *models.DeliveryError, got %T", err)
	}

	// Timeout errors should be retriable
	if !deliveryErr.IsRetriable() {
		t.Error("Expected timeout error to be retriable")
	}
}

func TestSendLead_InvalidPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)

	// Create a payload that cannot be marshaled to JSON
	payload := map[string]interface{}{
		"invalid": make(chan int), // channels cannot be marshaled to JSON
	}

	_, err := client.SendLead(context.Background(), payload)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error for invalid payload, got nil")
	}

	// Error should be a DeliveryError
	deliveryErr, ok := err.(*models.DeliveryError)
	if !ok {
		t.Fatalf("Expected *models.DeliveryError, got %T", err)
	}

	// Marshal errors should NOT be retriable (it's a client-side error)
	if deliveryErr.IsRetriable() {
		t.Error("Expected marshal error to be non-retriable")
	}
}

func TestSendLead_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)
	payload := map[string]interface{}{"phone": "1234567890"}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.SendLead(ctx, payload)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error for cancelled context, got nil")
	}

	// Error should be a DeliveryError
	deliveryErr, ok := err.(*models.DeliveryError)
	if !ok {
		t.Fatalf("Expected *models.DeliveryError, got %T", err)
	}

	// Context cancellation should be retriable (it's a transient error)
	if !deliveryErr.IsRetriable() {
		t.Error("Expected context cancellation to be retriable")
	}
}

func TestSendLead_RequestBodyParsing(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client := NewCustomerAPIClient(server.URL, "token", 30*time.Second)

	// Send a complex payload
	payload := map[string]interface{}{
		"phone": "1234567890",
		"email": "test@example.com",
		"product": map[string]interface{}{
			"name": "Solar Panels",
		},
		"attributes": map[string]interface{}{
			"roof_type":    "shingle",
			"roof_age":     10,
			"energy_usage": 1500.5,
		},
	}

	_, err := client.SendLead(context.Background(), payload)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the payload was sent correctly
	if receivedPayload["phone"] != "1234567890" {
		t.Errorf("Expected phone 1234567890, got %v", receivedPayload["phone"])
	}

	if receivedPayload["email"] != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %v", receivedPayload["email"])
	}

	product, ok := receivedPayload["product"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected product to be a map")
	}

	if product["name"] != "Solar Panels" {
		t.Errorf("Expected product name Solar Panels, got %v", product["name"])
	}
}
