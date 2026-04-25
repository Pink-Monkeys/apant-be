package worker

import (
	"context"

	"apant_be/internal/application/pentest"
	"apant_be/internal/infrastructure/queue"
)

type ScanWorker struct {
	service *pentest.Service
}

func NewScanWorker(service *pentest.Service) *ScanWorker {
	return &ScanWorker{service: service}
}

// Handle demonstrates async workflow consumption from queue payload.
func (w *ScanWorker) Handle(ctx context.Context, job queue.ScanJob) (pentest.AgentLoopResponse, error) {
	return w.service.AgentLoop(ctx, pentest.AgentChatRequest{
		SessionID: job.SessionID,
		Provider:  job.Provider,
		Model:     job.Model,
		Message:   job.Message,
		System:    job.System,
	})
}
