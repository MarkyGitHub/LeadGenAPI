package worker

import (
	"context"

	"github.com/checkfox/go_lead/internal/queue"
)

// ProcessJobForTest is a test helper that exposes the processLead method for integration tests
// This should only be used in tests
func (p *Processor) ProcessJobForTest(ctx context.Context, job *queue.Job) error {
	return p.processLead(ctx, job)
}
