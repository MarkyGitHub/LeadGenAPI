package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/checkfox/go_lead/internal/config"
	"github.com/google/uuid"
)

// AuthMiddleware provides authentication middleware for webhook endpoints
type AuthMiddleware struct {
	config *config.Config
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		config: cfg,
	}
}

// Authenticate validates the shared secret header if authentication is enabled
func (m *AuthMiddleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication if not enabled
		if !m.config.Auth.Enabled {
			next(w, r)
			return
		}
		
		// Generate correlation ID for logging
		correlationID := uuid.New().String()
		
		// Get shared secret from header
		providedSecret := r.Header.Get("X-Shared-Secret")
		
		// Validate shared secret
		if providedSecret == "" {
			log.Printf("[%s] Authentication failed: missing X-Shared-Secret header", correlationID)
			respondUnauthorized(w, correlationID, "missing authentication header")
			return
		}
		
		if providedSecret != m.config.Auth.SharedSecret {
			log.Printf("[%s] Authentication failed: invalid shared secret", correlationID)
			respondUnauthorized(w, correlationID, "invalid authentication credentials")
			return
		}
		
		// Authentication successful, proceed to next handler
		next(w, r)
	}
}

// respondUnauthorized sends a 401 Unauthorized response
func respondUnauthorized(w http.ResponseWriter, correlationID, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Correlation-ID", correlationID)
	w.WriteHeader(http.StatusUnauthorized)
	
	response := ErrorResponse{
		Error:         message,
		CorrelationID: correlationID,
	}
	
	// Encode response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[%s] Failed to encode unauthorized response: %v", correlationID, err)
	}
}

// RecoveryMiddleware recovers from panics and returns 500 Internal Server Error
type RecoveryMiddleware struct{}

// NewRecoveryMiddleware creates a new RecoveryMiddleware
func NewRecoveryMiddleware() *RecoveryMiddleware {
	return &RecoveryMiddleware{}
}

// Recover wraps a handler with panic recovery
func (m *RecoveryMiddleware) Recover(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				correlationID := uuid.New().String()
				log.Printf("[%s] Panic recovered: %v", correlationID, err)
				
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Correlation-ID", correlationID)
				w.WriteHeader(http.StatusInternalServerError)
				
				response := ErrorResponse{
					Error:         "internal server error",
					CorrelationID: correlationID,
				}
				
				if err := json.NewEncoder(w).Encode(response); err != nil {
					log.Printf("[%s] Failed to encode error response: %v", correlationID, err)
				}
			}
		}()
		
		next(w, r)
	}
}

