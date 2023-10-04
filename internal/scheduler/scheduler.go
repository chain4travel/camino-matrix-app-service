package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron"
)

var _ Scheduler = (*scheduler)(nil)

type Scheduler interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Schedule(period time.Duration, job func()) error
}

func New(ctx context.Context) Scheduler {
	return &scheduler{gocron.NewScheduler(time.UTC)}
}

type scheduler struct {
	*gocron.Scheduler
}

func (s *scheduler) Start(ctx context.Context) error {
	s.StartAsync()
	return nil
}

func (s *scheduler) Stop(ctx context.Context) error {
	s.Scheduler.Stop()
	return nil
}

func (s *scheduler) Schedule(period time.Duration, job func()) error {
	_, err := s.Every(period).Do(job)
	return err
}
