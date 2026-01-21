package queue

import "errors"

var (
	// ErrQueueUnavailable indicates the queue is not available
	ErrQueueUnavailable = errors.New("queue is unavailable")

	// ErrJobNotFound indicates the requested job was not found
	ErrJobNotFound = errors.New("job not found")

	// ErrInvalidPayload indicates the job payload is invalid
	ErrInvalidPayload = errors.New("invalid job payload")
)

// IsUnavailableError checks if an error indicates queue unavailability
func IsUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrQueueUnavailable)
}
