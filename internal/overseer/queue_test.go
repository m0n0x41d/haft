package overseer

import (
	"errors"
	"testing"
)

func TestEnqueueReviewJobIsDurableAndIdempotent(t *testing.T) {
	root := t.TempDir()
	packet := queueTestPacket(t)
	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-10T00:00:00Z"),
	}

	first, err := EnqueueReviewJob(root, stored, "2026-06-10T00:01:00Z")
	if err != nil {
		t.Fatalf("EnqueueReviewJob returned error: %v", err)
	}
	second, err := EnqueueReviewJob(root, stored, "2026-06-10T00:02:00Z")
	if err != nil {
		t.Fatalf("second EnqueueReviewJob returned error: %v", err)
	}
	if first.JobID != second.JobID {
		t.Fatalf("job ids differ: %q vs %q", first.JobID, second.JobID)
	}
	if second.Status != JobStatusPending {
		t.Fatalf("status = %q, want pending", second.Status)
	}

	jobs, err := ListReviewJobs(root)
	if err != nil {
		t.Fatalf("ListReviewJobs returned error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
}

func TestRequeueRunningReviewJobs(t *testing.T) {
	root := t.TempDir()
	packet := queueTestPacket(t)
	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-10T00:00:00Z"),
	}
	job, err := EnqueueReviewJob(root, stored, "2026-06-10T00:01:00Z")
	if err != nil {
		t.Fatalf("EnqueueReviewJob returned error: %v", err)
	}
	job.Status = JobStatusRunning
	if err := StoreReviewJob(root, job); err != nil {
		t.Fatalf("StoreReviewJob returned error: %v", err)
	}

	count, err := RequeueRunningReviewJobs(root, "restart", "2026-06-10T00:02:00Z")
	if err != nil {
		t.Fatalf("RequeueRunningReviewJobs returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("requeued = %d, want 1", count)
	}
	job, err = LoadReviewJob(root, job.JobID)
	if err != nil {
		t.Fatalf("LoadReviewJob returned error: %v", err)
	}
	if job.Status != JobStatusPending || job.LastError != "restart" {
		t.Fatalf("job = %+v, want pending with restart reason", job)
	}
}

func TestDaemonLeaseReportsRunningAndRejectsSecondLease(t *testing.T) {
	root := t.TempDir()
	lease, err := AcquireDaemonLease(root)
	if err != nil {
		t.Fatalf("AcquireDaemonLease returned error: %v", err)
	}
	defer lease.Close()

	status, err := LoadDaemonStatus(root)
	if err != nil {
		t.Fatalf("LoadDaemonStatus returned error: %v", err)
	}
	if !status.Running || status.PID == 0 {
		t.Fatalf("daemon status = %+v, want running pid", status)
	}

	_, err = AcquireDaemonLease(root)
	if !errors.Is(err, ErrDaemonAlreadyRunning) {
		t.Fatalf("second AcquireDaemonLease err = %v, want ErrDaemonAlreadyRunning", err)
	}
}

func queueTestPacket(t *testing.T) Packet {
	t.Helper()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}
	return packet
}
