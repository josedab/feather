package featherqlv2

import (
	"strings"
	"testing"
)

func TestQueryLifecycle(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())

	t.Run("submit", func(t *testing.T) {
		q, err := mgr.Submit("SELECT * FROM events TUMBLING(1m) EMIT CHANGES", "pipeline-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.State != QueryPending {
			t.Fatalf("expected pending state, got %s", q.State)
		}
		if q.PipelineID != "pipeline-1" {
			t.Fatalf("expected pipeline ID 'pipeline-1', got %s", q.PipelineID)
		}
	})

	t.Run("start", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-2")
		err := mgr.Start(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := mgr.Get(q.ID)
		if got.State != QueryRunning {
			t.Fatalf("expected running state, got %s", got.State)
		}
		if got.StartedAt == nil {
			t.Fatal("expected StartedAt to be set")
		}
	})

	t.Run("pause", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-3")
		mgr.Start(q.ID)
		err := mgr.Pause(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := mgr.Get(q.ID)
		if got.State != QueryPaused {
			t.Fatalf("expected paused state, got %s", got.State)
		}
	})

	t.Run("stop", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-4")
		mgr.Start(q.ID)
		err := mgr.Stop(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := mgr.Get(q.ID)
		if got.State != QueryStopped {
			t.Fatalf("expected stopped state, got %s", got.State)
		}
		if got.StoppedAt == nil {
			t.Fatal("expected StoppedAt to be set")
		}
	})

	t.Run("fail", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-5")
		mgr.Start(q.ID)
		err := mgr.Fail(q.ID, "connection lost")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := mgr.Get(q.ID)
		if got.State != QueryFailed {
			t.Fatalf("expected failed state, got %s", got.State)
		}
		if got.Error != "connection lost" {
			t.Fatalf("expected error 'connection lost', got %s", got.Error)
		}
	})

	t.Run("resume from paused", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-6")
		mgr.Start(q.ID)
		mgr.Pause(q.ID)
		err := mgr.Start(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := mgr.Get(q.ID)
		if got.State != QueryRunning {
			t.Fatalf("expected running state after resume, got %s", got.State)
		}
	})

	t.Run("invalid transitions", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-7")
		// Can't pause a pending query
		err := mgr.Pause(q.ID)
		if err == nil {
			t.Fatal("expected error pausing pending query")
		}
		// Can't start a stopped query
		mgr.Start(q.ID)
		mgr.Stop(q.ID)
		err = mgr.Start(q.ID)
		if err == nil {
			t.Fatal("expected error starting stopped query")
		}
	})
}

func TestRecordEvent(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())
	q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-1")
	mgr.Start(q.ID)

	mgr.RecordEvent(q.ID, false) // input
	mgr.RecordEvent(q.ID, false) // input
	mgr.RecordEvent(q.ID, true)  // output

	got, _ := mgr.Get(q.ID)
	if got.EventsIn != 2 {
		t.Fatalf("expected 2 events in, got %d", got.EventsIn)
	}
	if got.EventsOut != 1 {
		t.Fatalf("expected 1 event out, got %d", got.EventsOut)
	}
	if got.LastEventAt == nil {
		t.Fatal("expected LastEventAt to be set")
	}

	// Recording for nonexistent query should not panic
	mgr.RecordEvent("nonexistent", false)
}

func TestDeleteConstraints(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())

	t.Run("cannot delete running", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-1")
		mgr.Start(q.ID)
		err := mgr.Delete(q.ID)
		if err == nil {
			t.Fatal("expected error deleting running query")
		}
		if !strings.Contains(err.Error(), "running") {
			t.Fatalf("expected 'running' in error, got %v", err)
		}
	})

	t.Run("can delete stopped", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-2")
		mgr.Start(q.ID)
		mgr.Stop(q.ID)
		err := mgr.Delete(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("can delete failed", func(t *testing.T) {
		q, _ := mgr.Submit("SELECT * FROM s TUMBLING(1m) EMIT CHANGES", "p-3")
		mgr.Start(q.ID)
		mgr.Fail(q.ID, "error")
		err := mgr.Delete(q.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		err := mgr.Delete("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent query")
		}
	})
}

func TestMaxQueries(t *testing.T) {
	mgr := NewQueryManager(QueryManagerConfig{MaxQueries: 2})
	_, err := mgr.Submit("q1", "p-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = mgr.Submit("q2", "p-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = mgr.Submit("q3", "p-3")
	if err == nil {
		t.Fatal("expected error when max queries reached")
	}
}

func TestQueryManagerStats(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())

	q1, _ := mgr.Submit("q1", "p-1")
	mgr.Start(q1.ID)

	q2, _ := mgr.Submit("q2", "p-2")
	mgr.Start(q2.ID)
	mgr.Pause(q2.ID)

	q3, _ := mgr.Submit("q3", "p-3")
	mgr.Start(q3.ID)
	mgr.Stop(q3.ID)

	q4, _ := mgr.Submit("q4", "p-4")
	mgr.Start(q4.ID)
	mgr.Fail(q4.ID, "err")

	stats := mgr.Stats()
	if stats.TotalQueries != 4 {
		t.Fatalf("expected 4 total, got %d", stats.TotalQueries)
	}
	if stats.RunningQueries != 1 {
		t.Fatalf("expected 1 running, got %d", stats.RunningQueries)
	}
	if stats.PausedQueries != 1 {
		t.Fatalf("expected 1 paused, got %d", stats.PausedQueries)
	}
	if stats.StoppedQueries != 1 {
		t.Fatalf("expected 1 stopped, got %d", stats.StoppedQueries)
	}
	if stats.FailedQueries != 1 {
		t.Fatalf("expected 1 failed, got %d", stats.FailedQueries)
	}
}

func TestQueryList(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())
	mgr.Submit("q1", "p-1")
	mgr.Submit("q2", "p-2")

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(list))
	}
}

func TestNotFound(t *testing.T) {
	mgr := NewQueryManager(DefaultQueryManagerConfig())

	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent query")
	}

	err = mgr.Start("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent query")
	}

	err = mgr.Pause("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent query")
	}

	err = mgr.Stop("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent query")
	}

	err = mgr.Fail("nonexistent", "err")
	if err == nil {
		t.Fatal("expected error for nonexistent query")
	}
}
