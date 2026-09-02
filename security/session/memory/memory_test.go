package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/security"
)

func TestManager_CreateGetDelete(t *testing.T) {
	mgr := New()
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "USR001", &security.Principal{Subject: "USR001"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	got, err := mgr.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("expected session to be found, got %v", err)
	}
	if got.Principal.Subject != "USR001" {
		t.Fatalf("expected subject USR001, got %s", got.Principal.Subject)
	}

	if err := mgr.Delete(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Get(ctx, sess.ID)
	if err != security.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestManager_Expired(t *testing.T) {
	mgr := New()
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "USR001", &security.Principal{Subject: "USR001"}, -time.Second)
	if err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Get(ctx, sess.ID)
	if !errors.Is(err, security.ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestManager_GetMissing(t *testing.T) {
	mgr := New()
	_, err := mgr.Get(context.Background(), "does-not-exist")
	if !errors.Is(err, security.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr := New()
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "USR001", &security.Principal{Subject: "USR001"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := mgr.Get(ctx, sess.ID); err != nil {
				t.Errorf("concurrent get failed: %v", err)
			}
			if _, err := mgr.Refresh(ctx, sess.ID, time.Hour); err != nil {
				t.Errorf("concurrent refresh failed: %v", err)
			}
		}()
	}
	wg.Wait()
}
