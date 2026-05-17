package auditlog

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service is the default Recorder implementation. It persists entries in the
// background with a best-effort buffered channel — the Record method never
// blocks domain services. If the buffer fills (e.g. DB is down), entries are
// dropped and a warning is logged.
type Service struct {
	repo   Repository
	log    *zap.Logger
	buffer chan Entry
	batch  int
	flush  time.Duration

	// done is closed by FlushSync/Stop to signal run() to exit. stopOnce
	// ensures close(done) is called at most once regardless of how many times
	// Stop or FlushSync is invoked.
	done     chan struct{}
	stopOnce sync.Once
	// wg is Done when run() exits. FlushSync waits on it before starting its
	// own synchronous drain to prevent concurrent repo.Insert calls.
	wg sync.WaitGroup
}

func NewService(repo Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.With(zap.String("subsystem", "auditlog"))
	s := &Service{
		repo:   repo,
		log:    log,
		buffer: make(chan Entry, 1024),
		batch:  64,
		flush:  2 * time.Second,
		done:   make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Service) Record(ctx context.Context, actor *uuid.UUID, entity string, entityID *uuid.UUID, action string, meta map[string]any) {
	select {
	case <-s.done:
		// Service is stopped; silently drop.
		return
	default:
	}
	e := Entry{
		ActorID:    actor,
		EntityType: EntityType(entity),
		EntityID:   entityID,
		Action:     Action(action),
		Metadata:   meta,
		CreatedAt:  time.Now().UTC(),
	}
	select {
	case s.buffer <- e:
	default:
		s.log.Warn("auditlog: buffer full, dropping entry", zap.String("entity", entity), zap.String("action", action))
	}
}

func (s *Service) List(ctx context.Context, q Query) ([]Entry, error) {
	return s.repo.List(ctx, q)
}

func (s *Service) run() {
	defer s.wg.Done()
	t := time.NewTicker(s.flush)
	defer t.Stop()
	buf := make([]Entry, 0, s.batch)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if err := s.repo.Insert(context.Background(), buf); err != nil {
			s.log.Warn("auditlog: insert failed", zap.Error(err))
		}
		buf = buf[:0]
	}
	for {
		select {
		case <-s.done:
			// Drain whatever is still in the buffer before exiting.
			for {
				select {
				case e := <-s.buffer:
					buf = append(buf, e)
					if len(buf) >= s.batch {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case e := <-s.buffer:
			buf = append(buf, e)
			if len(buf) >= s.batch {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}

// Stop signals the background goroutine to flush and exit. Idempotent.
// Callers that need to wait for the final flush should use FlushSync instead.
func (s *Service) Stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

// FlushSync stops the background goroutine and then drains any remaining
// entries from the buffer synchronously. Idempotent — safe to call multiple
// times. The ctx controls only the final Insert call, not the drain loop.
func (s *Service) FlushSync(ctx context.Context) error {
	// Stop the background run() goroutine and wait for it to finish its own
	// drain. Without wg.Wait() here, run() and FlushSync could call
	// repo.Insert concurrently on the same entries (double-drain race, G-301).
	s.Stop()
	s.wg.Wait()

	buf := make([]Entry, 0, s.batch)
	for {
		select {
		case e := <-s.buffer:
			buf = append(buf, e)
		case <-ctx.Done():
			if len(buf) > 0 {
				// Best-effort flush of what we collected before context expired.
				_ = s.repo.Insert(context.Background(), buf)
			}
			return ctx.Err()
		default:
			if len(buf) > 0 {
				return s.repo.Insert(ctx, buf)
			}
			return nil
		}
	}
}
