package coordinator

import (
	"context"
	"errors"
)

var (
	// ErrQueueFull indicates the sync worker queue is saturated.
	ErrQueueFull = errors.New("sync queue is full")
	// ErrInvalidJob indicates caller provided an empty job.
	ErrInvalidJob = errors.New("invalid sync job")
)

// SyncJob represents one sync task to be executed by the single worker.
type SyncJob struct {
	Name    string
	Execute func(ctx context.Context) error
}

// SyncResult is emitted as event payload on completion or failure.
type SyncResult struct {
	JobName string
	Error   string
}

// SyncWorker enforces one-at-a-time sync execution.
type SyncWorker struct {
	state *AppState
	bus   *EventBus
	queue chan SyncJob
}

// NewSyncWorker creates a one-at-a-time worker with bounded queue.
func NewSyncWorker(queueSize int, state *AppState, bus *EventBus) *SyncWorker {
	if queueSize <= 0 {
		queueSize = 16
	}
	return &SyncWorker{
		state: state,
		bus:   bus,
		queue: make(chan SyncJob, queueSize),
	}
}

// Enqueue attempts to queue a sync job without blocking.
func (w *SyncWorker) Enqueue(job SyncJob) error {
	if job.Execute == nil || job.Name == "" {
		return ErrInvalidJob
	}

	select {
	case w.queue <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// Run processes jobs until ctx is cancelled.
func (w *SyncWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.queue:
			w.process(ctx, job)
		}
	}
}

func (w *SyncWorker) process(ctx context.Context, job SyncJob) {
	w.state.SetSyncing(true)
	w.bus.Emit(InternalEvent{Type: EvtSyncRequested, Payload: job.Name})

	err := job.Execute(ctx)
	if err != nil {
		w.bus.Emit(InternalEvent{
			Type: EvtSyncFailed,
			Payload: SyncResult{
				JobName: job.Name,
				Error:   err.Error(),
			},
		})
		w.state.SetSyncing(false)
		return
	}

	w.bus.Emit(InternalEvent{
		Type: EvtSyncCompleted,
		Payload: SyncResult{
			JobName: job.Name,
		},
	})
	w.state.SetSyncing(false)
}
