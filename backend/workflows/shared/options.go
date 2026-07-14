package shared

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// WithDefaultActivityOptions contains a set of default options for most/all
// temporal activities
func WithDefaultActivityOptions(ctx workflow.Context) workflow.Context {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Hour,
		},
	})
	return ctx
}

// WithScheduleToClose overrides the scheduleToCloseTimeout in a set of activity options
func WithScheduleToClose(ctx workflow.Context, duration time.Duration) workflow.Context {
	opts := workflow.GetActivityOptions(ctx)
	opts.ScheduleToCloseTimeout = duration
	return workflow.WithActivityOptions(ctx, opts)
}

// WithStartToCloseTimeout overrides the StartToCloseTimeout in a set of activity options
func WithStartToCloseTimeout(ctx workflow.Context, duration time.Duration) workflow.Context {
	opts := workflow.GetActivityOptions(ctx)
	opts.StartToCloseTimeout = duration
	return workflow.WithActivityOptions(ctx, opts)
}
