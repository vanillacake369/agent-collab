package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/store"
)

// testLock is a wrapper that implements store.Object
type testLock struct {
	*v1.Lock
}

func (t *testLock) DeepCopy() store.Object {
	if t == nil || t.Lock == nil {
		return nil
	}
	// Deep copy the lock
	copy := *t.Lock
	copy.ObjectMeta = t.ObjectMeta
	if t.Labels != nil {
		copy.Labels = make(map[string]string)
		for k, v := range t.Labels {
			copy.Labels[k] = v
		}
	}
	copy.Spec = t.Spec
	copy.Status = t.Status
	return &testLock{Lock: &copy}
}

func newTestLock(name string) *testLock {
	lock := v1.NewLock(name, v1.LockSpec{
		Target: v1.LockTarget{
			Type:     v1.LockTargetTypeFile,
			FilePath: "/path/to/" + name + ".go",
		},
		HolderID:  "agent-1",
		Intention: "Testing",
		TTL:       v1.Duration{Duration: 5 * time.Minute},
		Exclusive: true,
	})
	return &testLock{Lock: lock}
}

func TestStoreCreate(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()
	lock := newTestLock("test-lock")

	// Create should succeed
	err := s.Create(ctx, lock)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Resource version should be set
	if lock.ResourceVersion == "" {
		t.Error("ResourceVersion should be set after Create")
	}

	// Duplicate create should fail
	err = s.Create(ctx, newTestLock("test-lock"))
	if err != store.ErrAlreadyExists {
		t.Errorf("Create() duplicate error = %v, want %v", err, store.ErrAlreadyExists)
	}
}

func TestStoreGet(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()
	lock := newTestLock("test-lock")
	_ = s.Create(ctx, lock)

	// Get should return the item
	got, err := s.Get(ctx, "test-lock")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Name != "test-lock" {
		t.Errorf("Get() name = %s, want test-lock", got.Name)
	}

	// Get non-existent should return error
	_, err = s.Get(ctx, "non-existent")
	if err != store.ErrNotFound {
		t.Errorf("Get() non-existent error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestStoreUpdate(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()
	lock := newTestLock("test-lock")
	_ = s.Create(ctx, lock)

	// Get the current version
	got, _ := s.Get(ctx, "test-lock")
	oldVersion := got.ResourceVersion

	// Update should succeed
	got.Spec.Intention = "Updated intention"
	err := s.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Version should change
	updated, _ := s.Get(ctx, "test-lock")
	if updated.ResourceVersion == oldVersion {
		t.Error("ResourceVersion should change after Update")
	}
	if updated.Spec.Intention != "Updated intention" {
		t.Error("Update should persist changes")
	}

	// Update non-existent should fail
	nonExistent := newTestLock("non-existent")
	err = s.Update(ctx, nonExistent)
	if err != store.ErrNotFound {
		t.Errorf("Update() non-existent error = %v, want %v", err, store.ErrNotFound)
	}

	// Update with wrong version should fail
	stale := updated
	stale.ResourceVersion = oldVersion
	err = s.Update(ctx, stale)
	if err != store.ErrConflict {
		t.Errorf("Update() stale error = %v, want %v", err, store.ErrConflict)
	}
}

func TestStoreDelete(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()
	lock := newTestLock("test-lock")
	_ = s.Create(ctx, lock)

	// Delete should succeed
	err := s.Delete(ctx, "test-lock")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Get should fail after delete
	_, err = s.Get(ctx, "test-lock")
	if err != store.ErrNotFound {
		t.Errorf("Get() after delete error = %v, want %v", err, store.ErrNotFound)
	}

	// Delete again should fail
	err = s.Delete(ctx, "test-lock")
	if err != store.ErrNotFound {
		t.Errorf("Delete() again error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestStoreList(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()

	// Create multiple locks
	for i := 0; i < 5; i++ {
		lock := newTestLock("lock-" + string(rune('a'+i)))
		if i%2 == 0 {
			lock.Labels = map[string]string{"type": "even"}
		} else {
			lock.Labels = map[string]string{"type": "odd"}
		}
		_ = s.Create(ctx, lock)
	}

	// List all
	all, err := s.List(ctx, store.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 5 {
		t.Errorf("List() count = %d, want 5", len(all))
	}

	// List with selector
	even, err := s.List(ctx, store.ListOptions{
		LabelSelector: map[string]string{"type": "even"},
	})
	if err != nil {
		t.Fatalf("List() with selector error = %v", err)
	}
	if len(even) != 3 {
		t.Errorf("List() even count = %d, want 3", len(even))
	}

	// List with limit
	limited, err := s.List(ctx, store.ListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("List() with limit error = %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("List() limited count = %d, want 2", len(limited))
	}
}

func TestStoreWatch(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching
	watcher, err := s.Watch(ctx, store.WatchOptions{})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	defer watcher.Stop()

	// Create an item
	lock := newTestLock("watch-test")
	_ = s.Create(ctx, lock)

	// Should receive ADDED event
	select {
	case event := <-watcher.ResultChan():
		if event.Type != v1.EventAdded {
			t.Errorf("Watch event type = %s, want ADDED", event.Type)
		}
		if event.Object.Name != "watch-test" {
			t.Errorf("Watch event name = %s, want watch-test", event.Object.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for ADDED event")
	}

	// Update the item
	got, _ := s.Get(ctx, "watch-test")
	got.Spec.Intention = "Updated"
	_ = s.Update(ctx, got)

	// Should receive MODIFIED event
	select {
	case event := <-watcher.ResultChan():
		if event.Type != v1.EventModified {
			t.Errorf("Watch event type = %s, want MODIFIED", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for MODIFIED event")
	}

	// Delete the item
	_ = s.Delete(ctx, "watch-test")

	// Should receive DELETED event
	select {
	case event := <-watcher.ResultChan():
		if event.Type != v1.EventDeleted {
			t.Errorf("Watch event type = %s, want DELETED", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for DELETED event")
	}
}

func TestStoreWatchWithInitialEvents(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()

	// Create items before watching
	for i := 0; i < 3; i++ {
		lock := newTestLock("pre-" + string(rune('a'+i)))
		_ = s.Create(ctx, lock)
	}

	// Watch with initial events
	watcher, err := s.Watch(ctx, store.WatchOptions{
		SendInitialEvents: true,
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	defer watcher.Stop()

	// Should receive 3 ADDED events
	received := 0
	timeout := time.After(time.Second)
	for received < 3 {
		select {
		case event := <-watcher.ResultChan():
			if event.Type != v1.EventAdded {
				t.Errorf("Initial event type = %s, want ADDED", event.Type)
			}
			received++
		case <-timeout:
			t.Fatalf("Timeout waiting for initial events, received %d", received)
		}
	}
}

func TestStoreIndex(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()

	// Add an index by holder ID
	err := s.AddIndexer("byHolder", func(obj *testLock) []string {
		return []string{obj.Spec.HolderID}
	})
	if err != nil {
		t.Fatalf("AddIndexer() error = %v", err)
	}

	// Create items with different holders
	for i := 0; i < 6; i++ {
		lock := newTestLock("lock-" + string(rune('a'+i)))
		holder := "agent-1"
		if i >= 3 {
			holder = "agent-2"
		}
		lock.Spec.HolderID = holder
		_ = s.Create(ctx, lock)
	}

	// Query by index
	agent1Locks, err := s.ByIndex("byHolder", "agent-1")
	if err != nil {
		t.Fatalf("ByIndex() error = %v", err)
	}
	if len(agent1Locks) != 3 {
		t.Errorf("ByIndex(agent-1) count = %d, want 3", len(agent1Locks))
	}

	agent2Locks, err := s.ByIndex("byHolder", "agent-2")
	if err != nil {
		t.Fatalf("ByIndex() error = %v", err)
	}
	if len(agent2Locks) != 3 {
		t.Errorf("ByIndex(agent-2) count = %d, want 3", len(agent2Locks))
	}

	// Index keys
	keys, err := s.IndexKeys("byHolder")
	if err != nil {
		t.Fatalf("IndexKeys() error = %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("IndexKeys() count = %d, want 2", len(keys))
	}
}

func TestStoreConcurrency(t *testing.T) {
	s := New[*testLock]()
	defer s.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lock := newTestLock("concurrent-" + string(rune(i)))
			_ = s.Create(ctx, lock)
		}(i)
	}
	wg.Wait()

	// Should have all items
	if s.Len() != 100 {
		t.Errorf("Len() = %d, want 100", s.Len())
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = s.Get(ctx, "concurrent-"+string(rune(i)))
			_, _ = s.List(ctx, store.ListOptions{})
		}(i)
	}
	wg.Wait()
}

func TestStoreClose(t *testing.T) {
	s := New[*testLock]()

	ctx := context.Background()
	lock := newTestLock("test")
	_ = s.Create(ctx, lock)

	// Start a watcher
	watcher, _ := s.Watch(ctx, store.WatchOptions{})

	// Close the store
	err := s.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Operations should fail
	err = s.Create(ctx, newTestLock("new"))
	if err != store.ErrStoreClosed {
		t.Errorf("Create() after close error = %v, want %v", err, store.ErrStoreClosed)
	}

	// Watcher channel should be closed
	select {
	case _, ok := <-watcher.ResultChan():
		if ok {
			t.Error("Watcher channel should be closed after store close")
		}
	case <-time.After(100 * time.Millisecond):
		// Channel might already be closed
	}
}
