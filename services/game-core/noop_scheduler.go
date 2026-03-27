package main

import (
	"context"
	"time"

	"adplatform/internal/services/apigateway"
)

type noopGameScheduler struct{}

func (noopGameScheduler) Status() apigateway.GameSchedulerStatus {
	return apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 0}
}

func (noopGameScheduler) Start() (apigateway.GameSchedulerStatus, error) {
	return apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 0}, nil
}

func (noopGameScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	return apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 0}, nil
}

func (noopGameScheduler) Update(interval time.Duration) (apigateway.GameSchedulerStatus, error) {
	return apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 0}, nil
}

func (noopGameScheduler) Events(context.Context, apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	return apigateway.GameSchedulerEventPage{}, nil
}

func (noopGameScheduler) Close() error {
	return nil
}
