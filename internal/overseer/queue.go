package overseer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	ReviewJobSchemaVersion = "overseer.review_job.v1"

	JobStatusPending = "pending"
	JobStatusRunning = "running"
	JobStatusDone    = "done"
	JobStatusFailed  = "failed"
)

const (
	defaultReviewJobMaxAttempts = 3
	daemonPIDFile               = "daemon.pid"
	daemonLockDirName           = "daemon.lock"
)

var ErrDaemonAlreadyRunning = errors.New("overseer daemon already running")

type ReviewJob struct {
	SchemaVersion string `json:"schema_version"`
	JobID         string `json:"job_id"`
	ReviewRunID   string `json:"review_run_id"`
	PacketID      string `json:"packet_id"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	MaxAttempts   int    `json:"max_attempts"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	FinishedAt    string `json:"finished_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	LogPath       string `json:"log_path,omitempty"`
}

type QueueSummary struct {
	Pending int `json:"pending"`
	Running int `json:"running"`
	Done    int `json:"done"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

type DaemonStatus struct {
	Running bool         `json:"running"`
	PID     int          `json:"pid,omitempty"`
	Stale   bool         `json:"stale,omitempty"`
	Queue   QueueSummary `json:"queue"`
}

type DaemonLease struct {
	projectRoot string
	lockDir     string
}

func QueueDir(projectRoot string) string {
	return filepath.Join(OverseerDir(projectRoot), "queue")
}

func QueueJobsDir(projectRoot string) string {
	return filepath.Join(QueueDir(projectRoot), "jobs")
}

func ReviewLogPath(projectRoot string, reviewRunID string) string {
	return filepath.Join(OverseerDir(projectRoot), "logs", "review-"+reviewRunID+".log")
}

func DaemonLogPath(projectRoot string) string {
	return filepath.Join(OverseerDir(projectRoot), "logs", "daemon.log")
}

func EnqueueReviewJob(projectRoot string, stored StoredRun, now string) (ReviewJob, error) {
	jobID := reviewJobID(stored.Run.ReviewRunID)
	existing, err := LoadReviewJob(projectRoot, jobID)
	if err == nil {
		return existing, nil
	}
	if !os.IsNotExist(err) {
		return ReviewJob{}, err
	}

	job := ReviewJob{
		SchemaVersion: ReviewJobSchemaVersion,
		JobID:         jobID,
		ReviewRunID:   stored.Run.ReviewRunID,
		PacketID:      stored.Packet.PacketID,
		Status:        JobStatusPending,
		Attempts:      0,
		MaxAttempts:   defaultReviewJobMaxAttempts,
		CreatedAt:     strings.TrimSpace(now),
		UpdatedAt:     strings.TrimSpace(now),
		LogPath:       ReviewLogPath(projectRoot, stored.Run.ReviewRunID),
	}
	if err := StoreReviewJob(projectRoot, job); err != nil {
		return ReviewJob{}, err
	}
	return job, nil
}

func StoreReviewJob(projectRoot string, job ReviewJob) error {
	job = normalizeReviewJob(job)
	path := reviewJobPath(projectRoot, job.JobID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create review queue dir: %w", err)
	}
	if err := writeJSONFile(path, job); err != nil {
		return fmt.Errorf("write review queue job: %w", err)
	}
	return nil
}

func LoadReviewJob(projectRoot string, jobID string) (ReviewJob, error) {
	var job ReviewJob
	path := reviewJobPath(projectRoot, jobID)
	if err := readJSONFile(path, &job); err != nil {
		return ReviewJob{}, err
	}
	return normalizeReviewJob(job), nil
}

func ListReviewJobs(projectRoot string) ([]ReviewJob, error) {
	dir := QueueJobsDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ReviewJob{}, nil
		}
		return nil, fmt.Errorf("read review queue dir: %w", err)
	}

	jobs := make([]ReviewJob, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		job, err := LoadReviewJob(projectRoot, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	sort.SliceStable(jobs, func(i int, j int) bool {
		left := jobs[i]
		right := jobs[j]
		if left.CreatedAt == right.CreatedAt {
			return left.JobID < right.JobID
		}
		return left.CreatedAt < right.CreatedAt
	})
	return jobs, nil
}

func NextRunnableReviewJob(projectRoot string) (ReviewJob, bool, error) {
	jobs, err := ListReviewJobs(projectRoot)
	if err != nil {
		return ReviewJob{}, false, err
	}
	for _, job := range jobs {
		if job.Status == JobStatusPending {
			return job, true, nil
		}
	}
	return ReviewJob{}, false, nil
}

func RequeueRunningReviewJobs(projectRoot string, reason string, now string) (int, error) {
	jobs, err := ListReviewJobs(projectRoot)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, job := range jobs {
		if job.Status != JobStatusRunning {
			continue
		}
		job.Status = JobStatusPending
		job.LastError = strings.TrimSpace(reason)
		job.UpdatedAt = strings.TrimSpace(now)
		if err := StoreReviewJob(projectRoot, job); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func BuildQueueSummary(jobs []ReviewJob) QueueSummary {
	summary := QueueSummary{}
	for _, job := range jobs {
		switch normalizeJobStatus(job.Status) {
		case JobStatusPending:
			summary.Pending++
		case JobStatusRunning:
			summary.Running++
		case JobStatusDone:
			summary.Done++
		case JobStatusFailed:
			summary.Failed++
		}
		summary.Total++
	}
	return summary
}

func LoadDaemonStatus(projectRoot string) (DaemonStatus, error) {
	jobs, err := ListReviewJobs(projectRoot)
	if err != nil {
		return DaemonStatus{}, err
	}

	status := DaemonStatus{
		Queue: BuildQueueSummary(jobs),
	}

	pid, err := readDaemonPID(projectRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return DaemonStatus{}, err
	}

	status.PID = pid
	status.Running = processExists(pid)
	status.Stale = pid > 0 && !status.Running
	return status, nil
}

func AcquireDaemonLease(projectRoot string) (DaemonLease, error) {
	lockDir := filepath.Join(OverseerDir(projectRoot), daemonLockDirName)
	if err := os.MkdirAll(OverseerDir(projectRoot), 0o755); err != nil {
		return DaemonLease{}, fmt.Errorf("create overseer dir: %w", err)
	}

	if err := os.Mkdir(lockDir, 0o755); err != nil {
		if !os.IsExist(err) {
			return DaemonLease{}, fmt.Errorf("create daemon lock: %w", err)
		}

		status, statusErr := LoadDaemonStatus(projectRoot)
		if statusErr == nil && status.Running {
			return DaemonLease{}, ErrDaemonAlreadyRunning
		}
		if removeErr := os.RemoveAll(lockDir); removeErr != nil {
			return DaemonLease{}, fmt.Errorf("remove stale daemon lock: %w", removeErr)
		}
		if retryErr := os.Mkdir(lockDir, 0o755); retryErr != nil {
			if os.IsExist(retryErr) {
				return DaemonLease{}, ErrDaemonAlreadyRunning
			}
			return DaemonLease{}, fmt.Errorf("create daemon lock after stale cleanup: %w", retryErr)
		}
	}

	pid := os.Getpid()
	pidText := strconv.Itoa(pid) + "\n"
	pidPath := filepath.Join(OverseerDir(projectRoot), daemonPIDFile)
	if err := os.WriteFile(filepath.Join(lockDir, daemonPIDFile), []byte(pidText), 0o644); err != nil {
		_ = os.RemoveAll(lockDir)
		return DaemonLease{}, fmt.Errorf("write daemon lock pid: %w", err)
	}
	if err := os.WriteFile(pidPath, []byte(pidText), 0o644); err != nil {
		_ = os.RemoveAll(lockDir)
		return DaemonLease{}, fmt.Errorf("write daemon pid: %w", err)
	}

	return DaemonLease{projectRoot: projectRoot, lockDir: lockDir}, nil
}

func (lease DaemonLease) Close() error {
	var errs []error
	if lease.projectRoot != "" {
		errs = append(errs, os.Remove(filepath.Join(OverseerDir(lease.projectRoot), daemonPIDFile)))
	}
	if lease.lockDir != "" {
		errs = append(errs, os.RemoveAll(lease.lockDir))
	}
	for _, err := range errs {
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func StopDaemon(projectRoot string) error {
	pid, err := readDaemonPID(projectRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if pid > 0 && processExists(pid) {
		process, err := os.FindProcess(pid)
		if err == nil {
			_ = process.Signal(os.Interrupt)
		}
	}
	for attempt := 0; attempt < 20; attempt++ {
		if !processExists(pid) {
			return cleanupDaemonState(projectRoot)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func cleanupDaemonState(projectRoot string) error {
	pidPath := filepath.Join(OverseerDir(projectRoot), daemonPIDFile)
	lockDir := filepath.Join(OverseerDir(projectRoot), daemonLockDirName)
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(lockDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func reviewJobID(reviewRunID string) string {
	reviewRunID = strings.TrimSpace(reviewRunID)
	if strings.HasPrefix(reviewRunID, "rrun-") {
		return "ojob-" + strings.TrimPrefix(reviewRunID, "rrun-")
	}
	if reviewRunID == "" {
		return "ojob-unknown"
	}
	return "ojob-" + reviewRunID
}

func reviewJobPath(projectRoot string, jobID string) string {
	return filepath.Join(QueueJobsDir(projectRoot), strings.TrimSpace(jobID)+".json")
}

func normalizeReviewJob(job ReviewJob) ReviewJob {
	if strings.TrimSpace(job.SchemaVersion) == "" {
		job.SchemaVersion = ReviewJobSchemaVersion
	}
	job.JobID = strings.TrimSpace(job.JobID)
	job.ReviewRunID = strings.TrimSpace(job.ReviewRunID)
	job.PacketID = strings.TrimSpace(job.PacketID)
	job.Status = normalizeJobStatus(job.Status)
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = defaultReviewJobMaxAttempts
	}
	return job
}

func normalizeJobStatus(status string) string {
	switch strings.TrimSpace(status) {
	case JobStatusRunning:
		return JobStatusRunning
	case JobStatusDone:
		return JobStatusDone
	case JobStatusFailed:
		return JobStatusFailed
	default:
		return JobStatusPending
	}
}

func readDaemonPID(projectRoot string) (int, error) {
	data, err := os.ReadFile(filepath.Join(OverseerDir(projectRoot), daemonPIDFile))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse daemon pid: %w", err)
	}
	return pid, nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return process.Signal(syscall.Signal(0)) == nil
}
