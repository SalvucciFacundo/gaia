package cron

import (
	"context"
	"fmt"
	"log"

	"gaia/internal/core/domain"
)

// DeliveryService handles delivering cron job results to configured targets.
type DeliveryService struct {
	logger *log.Logger
}

// NewDeliveryService creates a delivery service with terminal output.
func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		logger: log.Default(),
	}
}

// Deliver sends a job result to the configured target.
// Currently supports "terminal" output; "telegram" is a future target.
func (d *DeliveryService) Deliver(ctx context.Context, job domain.CronJob, result domain.ToolResult) {
	switch job.DeliverTo {
	case "terminal", "":
		d.deliverTerminal(job, result)
	default:
		d.logger.Printf("cron: unknown delivery target %q for job %q, defaulting to terminal", job.DeliverTo, job.Name)
		d.deliverTerminal(job, result)
	}
}

// deliverTerminal prints the job result to stdout.
func (d *DeliveryService) deliverTerminal(job domain.CronJob, result domain.ToolResult) {
	fmt.Printf("\n=== Cron Job: %s ===\n", job.Name)
	fmt.Printf("Schedule: %s\n", job.Schedule)
	fmt.Printf("Task: %s\n", job.Task)
	if result.Success {
		fmt.Printf("Result: %s\n", result.Output)
	} else {
		fmt.Printf("Error: %s\n", result.Error)
	}
	fmt.Println("========================")
}
