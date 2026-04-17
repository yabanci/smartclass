package auditlog

import (
	"context"
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
}

func NewService(repo Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	s := &Service{
		repo:   repo,
		log:    log,
		buffer: make(chan Entry, 1024),
		batch:  64,
		flush:  2 * time.Second,
	}
	go s.run()
	return s
}

func (s *Service) Record(ctx context.Context, actor *uuid.UUID, entity string, entityID *uuid.UUID, action string, meta map[string]any) {
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
		case e, ok := <-s.buffer:
			if !ok {
				flush()
				return
			}
			buf = append(buf, e)
			if len(buf) >= s.batch {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}

// FlushSync drains the buffer synchronously. Intended for shutdown.
func (s *Service) FlushSync(ctx context.Context) error {
	// Drain quickly by reading non-blocking.
	buf := make([]Entry, 0, s.batch)
	for {
		select {
		case e := <-s.buffer:
			buf = append(buf, e)
		case <-ctx.Done():
			return ctx.Err()
		default:
			if len(buf) > 0 {
				return s.repo.Insert(ctx, buf)
			}
			return nil
		}
	}
}
