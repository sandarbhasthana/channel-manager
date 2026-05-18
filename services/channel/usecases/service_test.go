package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/channel-manager/channel-manager/services/channel/domain"
	"github.com/channel-manager/channel-manager/services/channel/usecases"
)

// ── in-file mocks ────────────────────────────────────────────────────────────

type mockConnRepo struct {
	create         func(context.Context, domain.Connection) (domain.Connection, error)
	getByID        func(context.Context, string) (domain.Connection, error)
	listByOrg      func(context.Context, string) ([]domain.Connection, error)
	updateStatus   func(context.Context, string, string, string) error
	updateName     func(context.Context, string, string) error
	updateLastSync func(context.Context, string, time.Time) error
	delete         func(context.Context, string) error
}

func (m *mockConnRepo) Create(ctx context.Context, c domain.Connection) (domain.Connection, error) {
	return m.create(ctx, c)
}
func (m *mockConnRepo) GetByID(ctx context.Context, id string) (domain.Connection, error) {
	return m.getByID(ctx, id)
}
func (m *mockConnRepo) ListByOrg(ctx context.Context, orgID string) ([]domain.Connection, error) {
	return m.listByOrg(ctx, orgID)
}
func (m *mockConnRepo) UpdateStatus(ctx context.Context, id, status, lastError string) error {
	return m.updateStatus(ctx, id, status, lastError)
}
func (m *mockConnRepo) UpdateName(ctx context.Context, id, name string) error {
	return m.updateName(ctx, id, name)
}
func (m *mockConnRepo) UpdateLastSync(ctx context.Context, id string, t time.Time) error {
	return m.updateLastSync(ctx, id, t)
}
func (m *mockConnRepo) Delete(ctx context.Context, id string) error { return m.delete(ctx, id) }

type mockChanRepo struct {
	create         func(context.Context, domain.Channel) (domain.Channel, error)
	getByID        func(context.Context, string) (domain.Channel, error)
	listByProperty func(context.Context, string) ([]domain.Channel, error)
	updateStatus   func(context.Context, string, string, string) error
	updateLastSync func(context.Context, string, time.Time) error
	delete         func(context.Context, string) error
}

func (m *mockChanRepo) Create(ctx context.Context, c domain.Channel) (domain.Channel, error) {
	return m.create(ctx, c)
}
func (m *mockChanRepo) GetByID(ctx context.Context, id string) (domain.Channel, error) {
	return m.getByID(ctx, id)
}
func (m *mockChanRepo) ListByProperty(ctx context.Context, propID string) ([]domain.Channel, error) {
	return m.listByProperty(ctx, propID)
}
func (m *mockChanRepo) UpdateStatus(ctx context.Context, id, status, lastError string) error {
	return m.updateStatus(ctx, id, status, lastError)
}
func (m *mockChanRepo) UpdateLastSync(ctx context.Context, id string, t time.Time) error {
	return m.updateLastSync(ctx, id, t)
}
func (m *mockChanRepo) Delete(ctx context.Context, id string) error { return m.delete(ctx, id) }

type mockSyncJobRepo struct {
	create                 func(context.Context, domain.SyncJob) error
	getByID                func(context.Context, string) (domain.SyncJob, error)
	updateStatus           func(context.Context, string, domain.SyncJobStatus, any, string) error
	listRecentByConnection func(context.Context, string, int32) ([]domain.SyncJob, error)
}

func (m *mockSyncJobRepo) Create(ctx context.Context, j domain.SyncJob) error {
	return m.create(ctx, j)
}
func (m *mockSyncJobRepo) GetByID(ctx context.Context, id string) (domain.SyncJob, error) {
	return m.getByID(ctx, id)
}
func (m *mockSyncJobRepo) UpdateStatus(ctx context.Context, id string, s domain.SyncJobStatus, r any, le string) error {
	return m.updateStatus(ctx, id, s, r, le)
}
func (m *mockSyncJobRepo) ListRecentByConnection(ctx context.Context, connectionID string, limit int32) ([]domain.SyncJob, error) {
	if m.listRecentByConnection != nil {
		return m.listRecentByConnection(ctx, connectionID, limit)
	}
	return nil, nil
}

type mockPublisher struct {
	publishSucceeded func(context.Context, domain.SyncJob) error
	publishFailed    func(context.Context, domain.SyncJob) error
}

func (m *mockPublisher) PublishSyncSucceeded(ctx context.Context, j domain.SyncJob) error {
	return m.publishSucceeded(ctx, j)
}
func (m *mockPublisher) PublishSyncFailed(ctx context.Context, j domain.SyncJob) error {
	return m.publishFailed(ctx, j)
}

type mockBreaker struct {
	allow func(string) (func(bool), error)
}

func (m *mockBreaker) Allow(key string) (func(bool), error) { return m.allow(key) }

// ── mock adapter ─────────────────────────────────────────────────────────────

type mockAdapter struct {
	id           string
	capabilities []domain.ChannelCapability
	pushAvail    func(context.Context, []domain.AvailabilityUpdate) error
}

func (a *mockAdapter) ChannelID() string                        { return a.id }
func (a *mockAdapter) Capabilities() []domain.ChannelCapability { return a.capabilities }
func (a *mockAdapter) PushAvailability(ctx context.Context, u []domain.AvailabilityUpdate) error {
	return a.pushAvail(ctx, u)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func noopPublisher() *mockPublisher {
	return &mockPublisher{
		publishSucceeded: func(_ context.Context, _ domain.SyncJob) error { return nil },
		publishFailed:    func(_ context.Context, _ domain.SyncJob) error { return nil },
	}
}

type mockSecrets struct {
	store   func(context.Context, string, map[string]string) (string, error)
	resolve func(context.Context, string) (map[string]string, error)
}

func (m *mockSecrets) Store(ctx context.Context, ref string, creds map[string]string) (string, error) {
	return m.store(ctx, ref, creds)
}
func (m *mockSecrets) Resolve(ctx context.Context, ref string) (map[string]string, error) {
	return m.resolve(ctx, ref)
}

func noopSecrets() *mockSecrets {
	return &mockSecrets{
		store:   func(_ context.Context, ref string, _ map[string]string) (string, error) { return ref, nil },
		resolve: func(_ context.Context, _ string) (map[string]string, error) { return map[string]string{}, nil },
	}
}

func noopBreaker() *mockBreaker {
	return &mockBreaker{allow: func(_ string) (func(bool), error) { return func(_ bool) {}, nil }}
}

func noopSyncJobRepo() *mockSyncJobRepo {
	return &mockSyncJobRepo{
		create:       func(_ context.Context, _ domain.SyncJob) error { return nil },
		getByID:      func(_ context.Context, _ string) (domain.SyncJob, error) { return domain.SyncJob{}, nil },
		updateStatus: func(_ context.Context, _ string, _ domain.SyncJobStatus, _ any, _ string) error { return nil },
		listRecentByConnection: func(_ context.Context, _ string, _ int32) ([]domain.SyncJob, error) {
			return nil, nil
		},
	}
}

func sampleConnection() domain.Connection {
	return domain.Connection{
		OrgID:    "org-1",
		Provider: "airbnb",
		Name:     "Airbnb Main",
		Status:   "active",
	}
}

// ── Connection CRUD tests ────────────────────────────────────────────────────

func TestCreateConnection(t *testing.T) {
	t.Run("happy path assigns ID and defaults status", func(t *testing.T) {
		repo := &mockConnRepo{
			create: func(_ context.Context, c domain.Connection) (domain.Connection, error) {
				if c.ID == "" {
					t.Fatal("expected ID to be set")
				}
				if c.Status != "active" {
					t.Fatalf("expected status=active, got %s", c.Status)
				}
				return c, nil
			},
		}
		svc := usecases.NewChannelService(repo, &mockChanRepo{}, nil, noopSecrets(), noopPublisher(), noopBreaker())
		conn, err := svc.CreateConnection(context.Background(), domain.Connection{
			OrgID:    "org-1",
			Provider: "airbnb",
			Name:     "Airbnb Main",
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn.ID == "" {
			t.Fatal("expected ID to be populated")
		}
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		repoErr := errors.New("unique constraint")
		repo := &mockConnRepo{
			create: func(_ context.Context, _ domain.Connection) (domain.Connection, error) {
				return domain.Connection{}, repoErr
			},
		}
		svc := usecases.NewChannelService(repo, &mockChanRepo{}, nil, noopSecrets(), noopPublisher(), noopBreaker())
		_, err := svc.CreateConnection(context.Background(), sampleConnection(), nil)
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected repoErr, got %v", err)
		}
	})
}

func TestGetConnection(t *testing.T) {
	t.Run("delegates to repo", func(t *testing.T) {
		want := sampleConnection()
		want.ID = "conn-1"
		repo := &mockConnRepo{
			getByID: func(_ context.Context, id string) (domain.Connection, error) {
				if id != "conn-1" {
					t.Fatalf("unexpected id: %s", id)
				}
				return want, nil
			},
		}
		svc := usecases.NewChannelService(repo, &mockChanRepo{}, nil, noopSecrets(), noopPublisher(), noopBreaker())
		got, err := svc.GetConnection(context.Background(), "conn-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != want.ID {
			t.Fatalf("got ID=%s, want %s", got.ID, want.ID)
		}
	})
}

func TestDeleteConnection(t *testing.T) {
	t.Run("delegates to repo", func(t *testing.T) {
		deleteCalled := false
		repo := &mockConnRepo{
			delete: func(_ context.Context, id string) error {
				deleteCalled = true
				if id != "conn-1" {
					t.Fatalf("unexpected id: %s", id)
				}
				return nil
			},
		}
		svc := usecases.NewChannelService(repo, &mockChanRepo{}, nil, noopSecrets(), noopPublisher(), noopBreaker())
		if err := svc.DeleteConnection(context.Background(), "conn-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleteCalled {
			t.Fatal("expected delete to be called")
		}
	})
}

// ── Channel CRUD tests ──────────────────────────────────────────────────────

func TestConnectChannel(t *testing.T) {
	t.Run("happy path verifies connection and derives provider", func(t *testing.T) {
		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, id string) (domain.Connection, error) {
				return domain.Connection{ID: id, Provider: "airbnb"}, nil
			},
		}
		chanRepo := &mockChanRepo{
			create: func(_ context.Context, ch domain.Channel) (domain.Channel, error) {
				if ch.Provider != "airbnb" {
					t.Fatalf("expected provider=airbnb, got %s", ch.Provider)
				}
				if ch.ID == "" {
					t.Fatal("expected ID to be assigned")
				}
				if ch.Status != "active" {
					t.Fatalf("expected status=active, got %s", ch.Status)
				}
				return ch, nil
			},
		}
		svc := usecases.NewChannelService(connRepo, chanRepo, nil, noopSecrets(), noopPublisher(), noopBreaker())
		ch, err := svc.ConnectChannel(context.Background(), domain.Channel{
			OrgID:              "org-1",
			PropertyID:         "prop-1",
			ConnectionID:       "conn-1",
			ExternalPropertyID: "ext-123",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch.Provider != "airbnb" {
			t.Fatalf("expected provider=airbnb, got %s", ch.Provider)
		}
	})

	t.Run("fails if connection not found", func(t *testing.T) {
		connErr := errors.New("not found")
		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{}, connErr
			},
		}
		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, nil, noopSecrets(), noopPublisher(), noopBreaker())
		_, err := svc.ConnectChannel(context.Background(), domain.Channel{ConnectionID: "bad-id"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPauseResumeChannel(t *testing.T) {
	t.Run("pause sets status to paused", func(t *testing.T) {
		statusSet := ""
		chanRepo := &mockChanRepo{
			updateStatus: func(_ context.Context, _ string, status string, _ string) error {
				statusSet = status
				return nil
			},
			getByID: func(_ context.Context, _ string) (domain.Channel, error) {
				return domain.Channel{ID: "ch-1", Status: "paused"}, nil
			},
		}
		svc := usecases.NewChannelService(&mockConnRepo{}, chanRepo, nil, noopSecrets(), noopPublisher(), noopBreaker())
		ch, err := svc.PauseChannel(context.Background(), "ch-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if statusSet != "paused" {
			t.Fatalf("expected statusSet=paused, got %s", statusSet)
		}
		if ch.Status != "paused" {
			t.Fatalf("expected ch.Status=paused, got %s", ch.Status)
		}
	})

	t.Run("resume sets status to active", func(t *testing.T) {
		statusSet := ""
		chanRepo := &mockChanRepo{
			updateStatus: func(_ context.Context, _ string, status string, _ string) error {
				statusSet = status
				return nil
			},
			getByID: func(_ context.Context, _ string) (domain.Channel, error) {
				return domain.Channel{ID: "ch-1", Status: "active"}, nil
			},
		}
		svc := usecases.NewChannelService(&mockConnRepo{}, chanRepo, nil, noopSecrets(), noopPublisher(), noopBreaker())
		ch, err := svc.ResumeChannel(context.Background(), "ch-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if statusSet != "active" {
			t.Fatalf("expected statusSet=active, got %s", statusSet)
		}
		if ch.Status != "active" {
			t.Fatalf("expected ch.Status=active, got %s", ch.Status)
		}
	})
}

func TestDisconnectChannel(t *testing.T) {
	t.Run("deletes and returns disconnected status", func(t *testing.T) {
		deleteCalled := false
		chanRepo := &mockChanRepo{
			getByID: func(_ context.Context, _ string) (domain.Channel, error) {
				return domain.Channel{ID: "ch-1", Status: "active"}, nil
			},
			delete: func(_ context.Context, id string) error {
				deleteCalled = true
				return nil
			},
		}
		svc := usecases.NewChannelService(&mockConnRepo{}, chanRepo, nil, noopSecrets(), noopPublisher(), noopBreaker())
		ch, err := svc.DisconnectChannel(context.Background(), "ch-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleteCalled {
			t.Fatal("expected delete to be called")
		}
		if ch.Status != "disconnected" {
			t.Fatalf("expected status=disconnected, got %s", ch.Status)
		}
	})
}

// ── Dispatch / sync tests ───────────────────────────────────────────────────

func TestPushAvailability(t *testing.T) {
	t.Run("happy path calls adapter and publishes success", func(t *testing.T) {
		adapterCalled := false
		publishedSuccess := false

		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{ID: "conn-1", OrgID: "org-1", Provider: "airbnb"}, nil
			},
			updateLastSync: func(_ context.Context, _ string, _ time.Time) error { return nil },
		}

		adapter := &mockAdapter{
			id:           "airbnb",
			capabilities: []domain.ChannelCapability{domain.CapabilityPushAvailability},
			pushAvail: func(_ context.Context, _ []domain.AvailabilityUpdate) error {
				adapterCalled = true
				return nil
			},
		}

		pub := &mockPublisher{
			publishSucceeded: func(_ context.Context, _ domain.SyncJob) error {
				publishedSuccess = true
				return nil
			},
			publishFailed: func(_ context.Context, _ domain.SyncJob) error { return nil },
		}

		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, noopSyncJobRepo(), noopSecrets(), pub, noopBreaker())
		svc.RegisterAdapter(adapter)

		err := svc.PushAvailability(context.Background(), "conn-1", []domain.AvailabilityUpdate{
			{PropertyID: "p1", RoomTypeID: "rt1", Available: 5},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !adapterCalled {
			t.Fatal("expected adapter to be called")
		}
		if !publishedSuccess {
			t.Fatal("expected success event to be published")
		}
	})

	t.Run("adapter error publishes failure", func(t *testing.T) {
		adapterErr := errors.New("airbnb api error")
		publishedFail := false

		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{ID: "conn-1", OrgID: "org-1", Provider: "airbnb"}, nil
			},
		}

		adapter := &mockAdapter{
			id:           "airbnb",
			capabilities: []domain.ChannelCapability{domain.CapabilityPushAvailability},
			pushAvail: func(_ context.Context, _ []domain.AvailabilityUpdate) error {
				return adapterErr
			},
		}

		pub := &mockPublisher{
			publishSucceeded: func(_ context.Context, _ domain.SyncJob) error { return nil },
			publishFailed: func(_ context.Context, _ domain.SyncJob) error {
				publishedFail = true
				return nil
			},
		}

		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, noopSyncJobRepo(), noopSecrets(), pub, noopBreaker())
		svc.RegisterAdapter(adapter)

		err := svc.PushAvailability(context.Background(), "conn-1", nil)
		if !errors.Is(err, adapterErr) {
			t.Fatalf("expected adapterErr, got %v", err)
		}
		if !publishedFail {
			t.Fatal("expected failure event to be published")
		}
	})

	t.Run("missing adapter returns error", func(t *testing.T) {
		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{ID: "conn-1", OrgID: "org-1", Provider: "unknown"}, nil
			},
		}
		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, noopSyncJobRepo(), noopSecrets(), noopPublisher(), noopBreaker())

		err := svc.PushAvailability(context.Background(), "conn-1", nil)
		if err == nil {
			t.Fatal("expected error for missing adapter")
		}
	})

	t.Run("unsupported capability returns ErrNotImplemented", func(t *testing.T) {
		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{ID: "conn-1", OrgID: "org-1", Provider: "airbnb"}, nil
			},
		}
		adapter := &mockAdapter{
			id:           "airbnb",
			capabilities: []domain.ChannelCapability{}, // no push_availability
		}
		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, noopSyncJobRepo(), noopSecrets(), noopPublisher(), noopBreaker())
		svc.RegisterAdapter(adapter)

		err := svc.PushAvailability(context.Background(), "conn-1", nil)
		if !errors.Is(err, domain.ErrNotImplemented) {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
	})

	t.Run("circuit breaker rejection returns error", func(t *testing.T) {
		connRepo := &mockConnRepo{
			getByID: func(_ context.Context, _ string) (domain.Connection, error) {
				return domain.Connection{ID: "conn-1", OrgID: "org-1", Provider: "airbnb"}, nil
			},
		}
		adapter := &mockAdapter{
			id:           "airbnb",
			capabilities: []domain.ChannelCapability{domain.CapabilityPushAvailability},
		}
		breaker := &mockBreaker{
			allow: func(_ string) (func(bool), error) {
				return nil, errors.New("circuit open")
			},
		}
		svc := usecases.NewChannelService(connRepo, &mockChanRepo{}, noopSyncJobRepo(), noopSecrets(), noopPublisher(), breaker)
		svc.RegisterAdapter(adapter)

		err := svc.PushAvailability(context.Background(), "conn-1", nil)
		if err == nil {
			t.Fatal("expected circuit breaker error")
		}
	})
}
