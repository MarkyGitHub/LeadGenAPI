package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkfox/go_lead/internal/config"
)

// Test authentication middleware when auth is disabled
func TestAuthMiddleware_Disabled(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	middleware := NewAuthMiddleware(cfg)

	// Create a test handler that should be called
	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	// Wrap with auth middleware
	wrappedHandler := middleware.Authenticate(testHandler)

	// Make request without auth header
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler(rr, req)

	// Handler should be called since auth is disabled
	if !handlerCalled {
		t.Error("Expected handler to be called when auth is disabled")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// Test authentication middleware with valid secret
func TestAuthMiddleware_ValidSecret(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:      true,
			SharedSecret: "test-secret-123",
		},
	}

	middleware := NewAuthMiddleware(cfg)

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := middleware.Authenticate(testHandler)

	// Make request with valid auth header
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Shared-Secret", "test-secret-123")
	rr := httptest.NewRecorder()

	wrappedHandler(rr, req)

	// Handler should be called with valid secret
	if !handlerCalled {
		t.Error("Expected handler to be called with valid secret")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// Test authentication middleware with missing secret
func TestAuthMiddleware_MissingSecret(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:      true,
			SharedSecret: "test-secret-123",
		},
	}

	middleware := NewAuthMiddleware(cfg)

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := middleware.Authenticate(testHandler)

	// Make request without auth header
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler(rr, req)

	// Handler should NOT be called
	if handlerCalled {
		t.Error("Expected handler NOT to be called with missing secret")
	}

	// Should return 401
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "missing authentication header" {
		t.Errorf("Expected error 'missing authentication header', got '%s'", response.Error)
	}
}

// Test authentication middleware with invalid secret
func TestAuthMiddleware_InvalidSecret(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Enabled:      true,
			SharedSecret: "test-secret-123",
		},
	}

	middleware := NewAuthMiddleware(cfg)

	handlerCalled := false
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := middleware.Authenticate(testHandler)

	// Make request with invalid auth header
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-Shared-Secret", "wrong-secret")
	rr := httptest.NewRecorder()

	wrappedHandler(rr, req)

	// Handler should NOT be called
	if handlerCalled {
		t.Error("Expected handler NOT to be called with invalid secret")
	}

	// Should return 401
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "invalid authentication credentials" {
		t.Errorf("Expected error 'invalid authentication credentials', got '%s'", response.Error)
	}
}

// Test recovery middleware
func TestRecoveryMiddleware_Panic(t *testing.T) {
	middleware := NewRecoveryMiddleware()

	// Create a handler that panics
	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}

	wrappedHandler := middleware.Recover(panicHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	// Should not panic, should return 500
	wrappedHandler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Check error response
	var response ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "internal server error" {
		t.Errorf("Expected error 'internal server error', got '%s'", response.Error)
	}

	if response.CorrelationID == "" {
		t.Error("Expected correlation_id to be set")
	}
}

// Test recovery middleware with normal execution
func TestRecoveryMiddleware_NormalExecution(t *testing.T) {
	middleware := NewRecoveryMiddleware()

	handlerCalled := false
	normalHandler := func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := middleware.Recover(normalHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler(rr, req)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}
