package cron

import (
	"context"
	"fmt"
	"log"

	"gaia/internal/core/domain"
)

// DeliveryService handles delivering cron job results to configured targets.
type DeliveryService struct {
	logger      *log.Logger
	gatewaySend func(ctx context.Context, platform, target, content string) error
}

// NewDeliveryService creates a delivery service with terminal output.
func NewDeliveryService() *DeliveryService {
	return &DeliveryService{
		logger: log.Default(),
	}
}

// SetGatewaySender configures a gateway send function for gateway delivery targets.
func (d *DeliveryService) SetGatewaySender(sendFn func(ctx context.Context, platform, target, content string) error) {
	d.gatewaySend = sendFn
}

// Deliver sends a job result to the configured target.
// Supports "terminal", "telegram", and "gateway" (platform:target format) delivery.
func (d *DeliveryService) Deliver(ctx context.Context, job domain.CronJob, result domain.ToolResult) {
	switch job.DeliverTo {
	case "terminal", "":
		d.deliverTerminal(job, result)
	case "gateway":
		d.deliverGateway(ctx, job, result)
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

// deliverGateway sends the result through the gateway to a configured platform+target.
// DeliverTarget format: "platform:target" (e.g., "telegram:123456789" or "discord:channel-id")
func (d *DeliveryService) deliverGateway(ctx context.Context, job domain.CronJob, result domain.ToolResult) {
	if d.gatewaySend == nil {
		d.logger.Printf("cron: gateway sender not configured for job %q", job.Name)
		d.deliverTerminal(job, result)
		return
	}

	// Parse DeliverTarget as "platform:target"
	platform := "telegram"
	target := job.DeliverTarget
	if idx := indexColon(job.DeliverTarget); idx > 0 {
		platform = job.DeliverTarget[:idx]
		target = job.DeliverTarget[idx+1:]
	}

	content := fmt.Sprintf("**%s** completed.\n%s", job.Name, result.Output)
	if !result.Success {
		content = fmt.Sprintf("**%s** failed.\nError: %s", job.Name, result.Error)
	}

	if err := d.gatewaySend(ctx, platform, target, content); err != nil {
		d.logger.Printf("cron: gateway delivery failed for job %q: %v", job.Name, err)
	}
}

// indexColon returns the index of the first colon in s, or -1 if none.
func indexColon(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}
