package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/channel-manager/channel-manager/services/inventory/domain"
	"github.com/channel-manager/channel-manager/services/inventory/usecases"
)

// ── in-file mocks ────────────────────────────────────────────────────────────

type mockRepo struct {
	listByRange func(context.Context, string, time.Time, time.Time) ([]domain.InventoryDay, error)
	upsertBatch func(context.Context, []domain.InventoryDay) error
}

func (m *mockRepo) ListByRange(ctx context.Context, id string, from, to time.Time) ([]domain.InventoryDay, error) {
	return m.listByRange(ctx, id, from, to)
}
func (m *mockRepo) UpsertBatch(ctx context.Context, days []domain.InventoryDay) error {
	return m.upsertBatch(ctx, days)
}

type mockIdem struct {
	exists func(context.Context, string) (bool, error)
	mark   func(context.Context, string) error
}

func (m *mockIdem) Exists(ctx context.Context, key string) (bool, error) { return m.exists(ctx, key) }
func (m *mockIdem) Mark(ctx context.Context, key string) error           { return m.mark(ctx, key) }

type mockPublisher struct {
	publish func(context.Context, []domain.InventoryDay) error
}

func (m *mockPublisher) PublishInventoryUpdated(ctx context.Context, days []domain.InventoryDay) error {
	return m.publish(ctx, days)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func noopPublisher() *mockPublisher {
	return &mockPublisher{publish: func(_ context.Context, _ []domain.InventoryDay) error { return nil }}
}

func noopIdem() *mockIdem {
	return &mockIdem{
		exists: func(_ context.Context, _ string) (bool, error) { return false, nil },
		mark:   func(_ context.Context, _ string) error { return nil },
	}
}

func sampleDays() []domain.InventoryDay {
	return []domain.InventoryDay{
		{OrgID: "org-1", RoomTypeID: "rt-1", StayDate: time.Now().UTC().Truncate(24 * time.Hour), Available: 5},
	}
}

// ── TestGetInventory ──────────────────────────────────────────────────────────

func TestGetInventory(t *testing.T) {
	t.Run("happy path returns repo result", func(t *testing.T) {
		want := sampleDays()
		repo := &mockRepo{
			listByRange: func(_ context.Context, _ string, _, _ time.Time) ([]domain.InventoryDay, error) {
				return want, nil
			},
		}
		svc := usecases.NewInventoryService(repo, noopIdem(), noopPublisher())
		got, err := svc.GetInventory(context.Background(), usecases.GetInventoryInput{
			RoomTypeID: "rt-1",
			From:       time.Now(),
			To:         time.Now().AddDate(0, 0, 7),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(want) {
			t.Fatalf("got %d days, want %d", len(got), len(want))
		}
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		repoErr := errors.New("db down")
		repo := &mockRepo{
			listByRange: func(_ context.Context, _ string, _, _ time.Time) ([]domain.InventoryDay, error) {
				return nil, repoErr
			},
		}
		svc := usecases.NewInventoryService(repo, noopIdem(), noopPublisher())
		_, err := svc.GetInventory(context.Background(), usecases.GetInventoryInput{RoomTypeID: "rt-1"})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected wrapped repoErr, got %v", err)
		}
	})
}

// ── TestBulkUpsertInventory ───────────────────────────────────────────────────

func TestBulkUpsertInventory(t *testing.T) {
	t.Run("happy path without idempotency key", func(t *testing.T) {
		upsertCalled := false
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error {
			upsertCalled = true
			return nil
		}}
		svc := usecases.NewInventoryService(repo, noopIdem(), noopPublisher())
		res, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{
			Days: sampleDays(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !upsertCalled {
			t.Fatal("expected repo.UpsertBatch to be called")
		}
		if res.RowsAffected != 1 {
			t.Fatalf("RowsAffected: got %d, want 1", res.RowsAffected)
		}
		if res.EventID == "" {
			t.Fatal("EventID must not be empty")
		}
	})

	t.Run("duplicate idempotency key returns ErrDuplicateRequest", func(t *testing.T) {
		upsertCalled := false
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error {
			upsertCalled = true
			return nil
		}}
		idem := &mockIdem{
			exists: func(_ context.Context, _ string) (bool, error) { return true, nil },
			mark:   func(_ context.Context, _ string) error { return nil },
		}
		svc := usecases.NewInventoryService(repo, idem, noopPublisher())
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{
			Days:           sampleDays(),
			IdempotencyKey: "key-abc",
		})
		if !errors.Is(err, usecases.ErrDuplicateRequest) {
			t.Fatalf("expected ErrDuplicateRequest, got %v", err)
		}
		if upsertCalled {
			t.Fatal("repo.UpsertBatch must not be called for a duplicate key")
		}
	})

	t.Run("new idempotency key: mark is called after successful upsert", func(t *testing.T) {
		markCalled := false
		idem := &mockIdem{
			exists: func(_ context.Context, _ string) (bool, error) { return false, nil },
			mark:   func(_ context.Context, _ string) error { markCalled = true; return nil },
		}
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error { return nil }}
		svc := usecases.NewInventoryService(repo, idem, noopPublisher())
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{
			Days:           sampleDays(),
			IdempotencyKey: "key-new",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !markCalled {
			t.Fatal("idem.Mark must be called after a successful upsert")
		}
	})

	t.Run("idempotency check error is returned before upsert", func(t *testing.T) {
		idemErr := errors.New("redis down")
		upsertCalled := false
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error {
			upsertCalled = true
			return nil
		}}
		idem := &mockIdem{
			exists: func(_ context.Context, _ string) (bool, error) { return false, idemErr },
			mark:   func(_ context.Context, _ string) error { return nil },
		}
		svc := usecases.NewInventoryService(repo, idem, noopPublisher())
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{
			Days:           sampleDays(),
			IdempotencyKey: "key-x",
		})
		if !errors.Is(err, idemErr) {
			t.Fatalf("expected wrapped idemErr, got %v", err)
		}
		if upsertCalled {
			t.Fatal("repo.UpsertBatch must not be called when idempotency check fails")
		}
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		repoErr := errors.New("constraint violation")
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error { return repoErr }}
		svc := usecases.NewInventoryService(repo, noopIdem(), noopPublisher())
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{Days: sampleDays()})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected wrapped repoErr, got %v", err)
		}
	})

	t.Run("event is published on success", func(t *testing.T) {
		publishCalled := false
		pub := &mockPublisher{publish: func(_ context.Context, _ []domain.InventoryDay) error {
			publishCalled = true
			return nil
		}}
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error { return nil }}
		svc := usecases.NewInventoryService(repo, noopIdem(), pub)
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{Days: sampleDays()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !publishCalled {
			t.Fatal("publisher.PublishInventoryUpdated must be called on success")
		}
	})

	t.Run("publisher error is non-fatal", func(t *testing.T) {
		pub := &mockPublisher{publish: func(_ context.Context, _ []domain.InventoryDay) error {
			return errors.New("bus unavailable")
		}}
		repo := &mockRepo{upsertBatch: func(_ context.Context, _ []domain.InventoryDay) error { return nil }}
		svc := usecases.NewInventoryService(repo, noopIdem(), pub)
		_, err := svc.BulkUpsertInventory(context.Background(), usecases.BulkUpsertInput{Days: sampleDays()})
		if err != nil {
			t.Fatalf("publisher error must be non-fatal, got: %v", err)
		}
	})
}
