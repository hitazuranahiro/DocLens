package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tomeku/doclens/services/shared/jobs"
	"github.com/tomeku/doclens/services/shared/jobs/inmem"
)

func TestInmemBus_RecordsTasks(t *testing.T) {
	bus := inmem.NewBus()
	ctx := context.Background()

	r1, err := bus.Enqueue(ctx, jobs.Task{Type: "extract.document", Payload: map[string]string{"id": "doc-1"}})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if r1.TaskID == "" {
		t.Fatal("receipt is missing a task id")
	}
	if r1.Queue != "default" {
		t.Fatalf("queue = %q, want default", r1.Queue)
	}

	r2, err := bus.Enqueue(ctx, jobs.Task{Type: "extract.document", Queue: "priority", Payload: 42})
	if err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if r2.Queue != "priority" {
		t.Fatalf("queue = %q, want priority", r2.Queue)
	}

	tasks := bus.Tasks()
	if len(tasks) != 2 {
		t.Fatalf("recorded %d tasks, want 2", len(tasks))
	}
	if tasks[0].Type != "extract.document" {
		t.Fatalf("type[0] = %q", tasks[0].Type)
	}
}

func TestInmemBus_UniqueDedup(t *testing.T) {
	bus := inmem.NewBus()
	ctx := context.Background()
	common := jobs.Task{Type: "extract.document", UniqueKey: "doc-1"}

	if _, err := bus.Enqueue(ctx, common); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if _, err := bus.Enqueue(ctx, common); !errors.Is(err, jobs.ErrDuplicate) {
		t.Fatalf("err = %v, want ErrDuplicate", err)
	}
}

func TestInmemBus_FailOnceIsolatesNextCall(t *testing.T) {
	bus := inmem.NewBus()
	ctx := context.Background()

	bus.FailOnce(errors.New("boom"))
	if _, err := bus.Enqueue(ctx, jobs.Task{Type: "extract.document"}); err == nil {
		t.Fatal("expected forced failure")
	}
	if _, err := bus.Enqueue(ctx, jobs.Task{Type: "extract.document"}); err != nil {
		t.Fatalf("subsequent enqueue should succeed, got %v", err)
	}
}
