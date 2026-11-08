package runs

import (
	"errors"
	"testing"
	"time"
)

func mustTime() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestAcquireBusyFinalizeRecent(t *testing.T) {
	store := New(t.TempDir())

	rec, release, err := store.Acquire("DailyAll", "配置 1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusRunning || rec.Task != "DailyAll" {
		t.Fatalf("record = %+v", rec)
	}
	if _, _, err := store.Acquire("Other", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Acquire = %v, want ErrBusy", err)
	}
	if active, ok := store.Active(); !ok || active.ID != rec.ID {
		t.Fatalf("Active = %+v, %v", active, ok)
	}

	if err := store.Finalize(rec, StatusSuccess, 0, nil); err != nil {
		t.Fatal(err)
	}
	release()
	if _, ok := store.Active(); ok {
		t.Fatal("lock should be released")
	}

	got, err := store.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusSuccess || got.FinishedAt == nil || got.DurationS < 0 {
		t.Fatalf("finalized = %+v", got)
	}

	recent, err := store.Recent(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].ID != rec.ID {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestRecentSkipsDotFilesAndOrdersDesc(t *testing.T) {
	store := New(t.TempDir())
	for _, id := range []string{"20260101-000001-1", "20260102-000001-1"} {
		r := &Record{ID: id, Status: StatusSuccess, StartedAt: mustTime()}
		if err := store.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	recent, err := store.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].ID != "20260102-000001-1" {
		t.Fatalf("recent = %+v", recent)
	}
}

func TestNewRecordIDsAreUnique(t *testing.T) {
	store := New(t.TempDir())
	a := store.NewRecord()
	if err := store.Save(a); err != nil {
		t.Fatal(err)
	}
	b := store.NewRecord()
	if a.ID == b.ID {
		t.Fatalf("duplicate id %q", a.ID)
	}
}
