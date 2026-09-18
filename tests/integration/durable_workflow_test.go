//go:build integration

package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sentiolabs/arc/internal/storage"
	"github.com/sentiolabs/arc/internal/types"
	"github.com/stretchr/testify/require"
)

// The integration package is serial. Select an owned process for this test and
// restore the harness address afterward; no test restarts the shared server.
type durableFixture struct {
	t                                                  *testing.T
	home, dir, config, db, root, url, project, session string
	process                                            *exec.Cmd
	log                                                *os.File
}

func newDurableFixture(t *testing.T) *durableFixture {
	t.Helper()
	return newDurableFixtureBinary(t, arcBinary)
}

func newDurableFixtureBinary(t *testing.T, binary string) *durableFixture {
	t.Helper()
	f := &durableFixture{t: t, home: setupHome(t), dir: t.TempDir(), session: "durable-review-session"}
	f.db = filepath.Join(f.home, "data.db")
	f.root = filepath.Join(f.home, "server-content")
	f.config = filepath.Join(f.home, "isolated.toml")
	socket, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := socket.Addr().(*net.TCPAddr).Port
	require.NotEqual(t, 7432, port)
	require.NoError(t, socket.Close())
	f.url = fmt.Sprintf("http://127.0.0.1:%d", port)
	require.NoError(t, os.WriteFile(f.config, fmt.Appendf(nil,
		"[server]\nport = %d\ndb_path = %q\nplans_dir = %q\n[plans]\ndir = %q\ntype = \"markdown\"\n", port, f.db, f.root, filepath.Join(f.dir, "drafts")), 0o600))
	old := serverURL
	serverURL = f.url
	t.Cleanup(func() { serverURL = old })
	t.Cleanup(f.stop)
	f.start(binary)
	var project types.Project
	decodeDurable(t, f.cli("project", "create", "durable-workflow", "--path", f.dir, "--json"), &project)
	f.project = project.ID
	require.NotEmpty(t, f.project)
	f.cli("paths", "add", f.dir, "--json")
	return f
}

func (f *durableFixture) start(binary string) {
	f.t.Helper()
	var err error
	f.log, err = os.CreateTemp(f.home, "server-*.log")
	require.NoError(f.t, err)
	port := strings.TrimPrefix(f.url, "http://127.0.0.1:")
	f.process = exec.Command(binary, "--config", f.config, "server", "start", "--foreground", "--port", port, "--db", f.db)
	f.process.Dir = f.home
	f.process.Env = append(os.Environ(), "HOME="+f.home, "XDG_CONFIG_HOME="+filepath.Join(f.home, "config-home"), "ARC_SERVER="+f.url, "ARC_SESSION_ID=")
	f.process.Stdout = f.log
	f.process.Stderr = f.log
	require.NoError(f.t, f.process.Start())
	if err := waitForServer(f.url, 15*time.Second); err != nil {
		data, _ := os.ReadFile(f.log.Name())
		f.t.Fatalf("owned server: %v\n%s", err, data)
	}
}

func (f *durableFixture) stop() {
	if f.process == nil {
		return
	}
	// Only this fixture's foreground PID is signalled; never use daemon stop/restart.
	_ = f.process.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- f.process.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = f.process.Process.Kill()
		<-done
	}
	_ = f.log.Close()
	f.process = nil
}

func (f *durableFixture) run(input string, args ...string) (string, error) {
	f.t.Helper()
	// Reuse the harness's isolated HOME/server/stdin helper. Explicit config and
	// project avoid workspace discovery and inherited user configuration.
	all := []string{"--config", f.config, "--server", f.url}
	if f.project != "" {
		all = append(all, "--project", f.project)
	}
	all = append(all, args...)
	f.t.Setenv("ARC_SESSION_ID", f.session)
	return arcCmdInDirWithStdin(f.t, f.home, f.dir, input, all...)
}

func (f *durableFixture) cli(args ...string) string {
	f.t.Helper()
	out, err := f.run("", args...)
	require.NoError(f.t, err, "arc %v: %s", args, out)
	return out
}

func (f *durableFixture) failure(args ...string) string {
	f.t.Helper()
	out, err := f.run("", args...)
	require.Error(f.t, err, "arc %v: %s", args, out)
	return out
}

func decodeDurable(t *testing.T, body string, target any) {
	t.Helper()
	require.NoError(t, json.Unmarshal([]byte(body), target), body)
}

func (f *durableFixture) file(name string, body any) string {
	f.t.Helper()
	var data []byte
	if s, ok := body.(string); ok {
		data = []byte(s)
	} else {
		var err error
		data, err = json.Marshal(body)
		require.NoError(f.t, err)
	}
	path := filepath.Join(f.dir, name)
	require.NoError(f.t, os.WriteFile(path, data, 0o600))
	return path
}

func (f *durableFixture) createPlan(name, content string) storage.PlanWriteResult {
	f.t.Helper()
	path := f.file(name+".md", content)
	var result storage.PlanWriteResult
	decodeDurable(f.t, f.cli("plan", "create", path, "--no-frontmatter", "--idempotency-key", name, "--json"), &result)
	require.Equal(f.t, content, result.Revision.Content)
	require.Equal(f.t, fmt.Sprintf("%x", sha256.Sum256([]byte(content))), result.Revision.ContentSHA256)
	require.Equal(f.t, int64(len(content)), result.Revision.ContentBytes)
	require.NoError(f.t, os.Remove(path))
	return result
}

func (f *durableFixture) review(plan string, revision int) {
	f.t.Helper()
	n := strconv.Itoa(revision)
	for _, decision := range []string{"submit", "approve"} {
		capture := filepath.Join(f.dir, plan+"-"+n+"-"+decision+".json")
		f.cli("plan", "show", plan, "--revision", n, "--review-context-output", capture, "--json")
		f.cli("plan", decision, plan, "--revision", n, "--review-context", capture, "--json")
	}
	var events []storage.PlanReviewEvent
	decodeDurable(f.t, string(durableAPI(f.t, "GET", "/api/v1/projects/"+f.project+"/plans/"+plan+"/revisions/"+n+"/decisions", "", nil, 200)), &events)
	require.Len(f.t, events, 2)
	for _, event := range events {
		require.Equal(f.t, "cli", event.Actor)
		require.Equal(f.t, f.session, event.SessionID)
		require.Equal(f.t, int64(revision), event.Revision)
	}
}

func (f *durableFixture) issue(title, kind, parent string) types.IssueDetails {
	args := []string{"create", title, "--type", kind, "--json"}
	if parent != "" {
		args = append(args, "--parent", parent)
	}
	var issue types.IssueDetails
	decodeDurable(f.t, f.cli(args...), &issue)
	return issue
}

func (f *durableFixture) show(id string) types.IssueDetails {
	f.t.Helper()
	var issue types.IssueDetails
	decodeDurable(f.t, f.cli("show", id, "--json"), &issue)
	return issue
}

func (f *durableFixture) proposal(container, plan string, revision int) storage.PlanAdoptionResult {
	f.t.Helper()
	var preview storage.PlanAdoptionResult
	decodeDurable(f.t, f.cli("plan", "adopt", container, plan, "--revision", strconv.Itoa(revision), "--dry-run", "--json"), &preview)
	preview.Request.Tasks = nil
	for _, change := range preview.Tasks {
		preview.Request.Tasks = append(preview.Request.Tasks, types.ReconciledTask{
			IssueID: change.Before.IssueID, Expected: change.Before.Expected,
			Disposition: "unchanged", Reason: "Task retains history under the complete proposed chain",
			FollowUpKeys: []string{},
		})
	}
	for _, problem := range preview.Errors {
		if problem.Code != "missing_container_pin" {
			continue
		}
		child := f.show(problem.IssueID)
		preview.Request.ContainerPins = append(preview.Request.ContainerPins, types.ReconciledContainerPin{
			ContainerID: child.ID, ExpectedContainerVersion: child.ContractVersion,
			ExpectedPin: child.GoverningPlan, TargetPin: child.GoverningPlan,
			Disposition: "compatible", Reason: "Tactical retention remains compatible with the architecture",
		})
	}
	return preview
}

func (f *durableFixture) apply(container, plan string, revision int, request types.PlanAdoptionRequest, key string) (storage.PlanAdoptionResult, string) {
	f.t.Helper()
	manifest := f.file(key+".json", request)
	args := []string{"plan", "adopt", container, plan, "--revision", strconv.Itoa(revision), "--reconciliation", manifest, "--json"}
	var checked storage.PlanAdoptionResult
	decodeDurable(f.t, f.cli(append(args, "--dry-run")...), &checked)
	require.Empty(f.t, checked.Errors)
	var applied storage.PlanAdoptionResult
	decodeDurable(f.t, f.cli(append(args, "--idempotency-key", key)...), &applied)
	require.False(f.t, applied.DryRun)
	require.Equal(f.t, "cli", applied.Actor)
	require.Equal(f.t, f.session, applied.SessionID)
	return applied, manifest
}

func (f *durableFixture) claim(id, name string) (types.ExpectedGovernance, string) {
	f.t.Helper()
	path := filepath.Join(f.dir, name+".json")
	f.cli("update", id, "--take", "--context-output", path, "--json")
	body, err := os.ReadFile(path)
	require.NoError(f.t, err)
	var context types.ExpectedGovernance
	decodeDurable(f.t, string(body), &context)
	return context, path
}

func (f *durableFixture) evidence(id, path, phase string) storage.ExecutionEvidence {
	f.t.Helper()
	out, err := f.run("Verified exact captured chain and retained bytes", "evidence", id, "--phase", phase, "--context", path, "--stdin", "--json")
	require.NoError(f.t, err, out)
	var evidence storage.ExecutionEvidence
	decodeDurable(f.t, out, &evidence)
	require.Equal(f.t, "cli", evidence.Actor)
	require.Equal(f.t, f.session, evidence.SessionID)
	return evidence
}

func TestDurableLayeredCLIWorkflow(t *testing.T) {
	f := newDurableFixture(t)
	metaBytes := "\ufeff# Architecture\r\nRetain all history.\n"
	meta := f.createPlan("architecture", metaBytes)
	f.review(meta.Plan.ID, 1)
	milestone := f.issue("Architecture milestone", "milestone", "")
	preview := f.proposal(milestone.ID, meta.Plan.ID, 1)
	f.apply(milestone.ID, meta.Plan.ID, 1, preview.Request, "attach-meta")
	tactical := f.createPlan("tactical", "# Tactical\nKeep immutable files and review history.\n")
	f.review(tactical.Plan.ID, 1)
	epic := f.issue("Tactical epic", "epic", milestone.ID)
	preview = f.proposal(epic.ID, tactical.Plan.ID, 1)
	f.apply(epic.ID, tactical.Plan.ID, 1, preview.Request, "attach-tactical")
	task := f.issue("Implement retention", "task", epic.ID)
	f.session = "durable-worker-before"
	before, oldCapture := f.claim(task.ID, "before")
	require.Equal(t, types.PlanReference{PlanID: tactical.Plan.ID, Revision: 1}, before.Governing.Reference)
	require.Equal(t, epic.ID, before.Governing.ContainerID)
	require.Equal(t, []types.GoverningPlanContext{{
		ContainerID: milestone.ID, ContainerType: types.TypeMilestone,
		Reference: types.PlanReference{PlanID: meta.Plan.ID, Revision: 1},
	}}, before.Governing.Context)
	originalEvidence := f.evidence(task.ID, oldCapture, "build")
	require.Equal(t, before, originalEvidence.Expected)
	var comment types.PlanComment
	decodeDurable(t, f.cli("plan", "comment", "create", meta.Plan.ID, "--revision", "1", "--content", "Explain retention after restart", "--line", "2", "--json"), &comment)
	require.Equal(t, int64(1), *comment.Revision)
	f.session = "durable-review-after"
	newer := f.file("architecture-v2.md", "# Architecture v2\nRetain history through restart and recovery.\n")
	f.cli("plan", "update", meta.Plan.ID, newer, "--expected-revision", "1", "--idempotency-key", "meta-v2", "--json")
	f.cli("plan", "feedback", meta.Plan.ID, "--revision", "2", "--comment", comment.ID,
		"--expected-comment-version", strconv.FormatInt(comment.Version, 10),
		"--expected-feedback-version", "1", "--disposition", "addressed",
		"--reason", "Revision 2 explicitly requires restart retention", "--json")
	f.review(meta.Plan.ID, 2)
	tacticalNext := f.file("tactical-v2.md", "# Tactical v2\nVerify retained bytes after restarting the server.\n")
	f.cli("plan", "update", tactical.Plan.ID, tacticalNext, "--expected-revision", "1", "--idempotency-key", "tactical-v2", "--json")
	f.review(tactical.Plan.ID, 2)
	preview = f.proposal(milestone.ID, meta.Plan.ID, 2)
	var runningTasks []string
	for _, problem := range preview.Errors {
		if problem.Code == "task_in_progress" {
			runningTasks = append(runningTasks, problem.IssueID)
		}
	}
	require.Contains(t, runningTasks, task.ID, "preview must identify the affected running task")
	runningManifest := f.file("running-reconciliation.json", preview.Request)
	var runningPreview storage.PlanAdoptionResult
	decodeDurable(t, f.cli("plan", "adopt", milestone.ID, meta.Plan.ID, "--revision", "2",
		"--reconciliation", runningManifest, "--dry-run", "--json"), &runningPreview)
	require.Len(t, runningPreview.Errors, 1, "complete coverage must leave only the pause violation")
	require.Equal(t, "task_in_progress", runningPreview.Errors[0].Code)
	require.Equal(t, task.ID, runningPreview.Errors[0].IssueID)
	beforeRejectedApply := []types.IssueDetails{f.show(task.ID), f.show(epic.ID), f.show(milestone.ID)}
	require.Contains(t, f.failure("plan", "adopt", milestone.ID, meta.Plan.ID, "--revision", "2",
		"--reconciliation", runningManifest, "--idempotency-key", "reject-running"), "409")
	for _, before := range beforeRejectedApply {
		after := f.show(before.ID)
		require.Equal(t, before.ContractVersion, after.ContractVersion)
		require.Equal(t, before.GoverningPlan, after.GoverningPlan)
		require.Equal(t, before.Status, after.Status)
		require.Equal(t, before.Description, after.Description)
	}
	// Pause actual execution before changing status. The fixture has no worker thread.
	f.cli("update", task.ID, "--status", "open", "--json")
	preview = f.proposal(milestone.ID, meta.Plan.ID, 2)
	require.Len(t, preview.Request.Tasks, 1)
	require.Len(t, preview.Request.ContainerPins, 1)
	newDescription := "Verify restart retention against both revision 2 designs."
	preview.Request.Tasks[0].Disposition = "updated"
	preview.Request.Tasks[0].Description = &newDescription
	preview.Request.Tasks[0].Reason = "Add the restart acceptance check required by architecture v2"
	preview.Request.ContainerPins[0].Disposition = "updated"
	preview.Request.ContainerPins[0].TargetPin = &types.PlanReference{PlanID: tactical.Plan.ID, Revision: 2}
	preview.Request.ContainerPins[0].Reason = "Tactical v2 implements the architecture v2 restart guarantee"
	f.session = "durable-adoption-session"
	adoption, manifest := f.apply(milestone.ID, meta.Plan.ID, 2, preview.Request, "adopt-both-v2")
	require.Equal(t, newDescription, f.show(task.ID).Description)
	require.Equal(t, int64(2), f.show(epic.ID).GoverningPlan.Revision)
	require.Equal(t, int64(2), f.show(milestone.ID).GoverningPlan.Revision)
	f.session = "different-replay-session"
	var replay storage.PlanAdoptionResult
	decodeDurable(t, f.cli("plan", "adopt", milestone.ID, meta.Plan.ID, "--revision", "2", "--reconciliation", manifest, "--idempotency-key", "adopt-both-v2", "--json"), &replay)
	require.True(t, replay.Replay)
	replay.Replay = false
	require.Equal(t, adoption, replay, "replay must retain original actor, session, IDs and snapshots")
	var adoptions []storage.PlanAdoptionResult
	decodeDurable(t, string(durableAPI(t, "GET", "/api/v1/projects/"+f.project+"/issues/"+milestone.ID+"/plan-adoptions", "", nil, 200)), &adoptions)
	require.Len(t, adoptions, 2)
	require.Contains(t, adoptions, adoption)
	unchanged := f.show(task.ID)
	require.Contains(t, f.failure("close", task.ID, "--context", oldCapture), "409")
	require.Contains(t, f.failure("update", task.ID, "--status", "closed", "--context", oldCapture), "409")
	out, err := f.run("stale work", "evidence", task.ID, "--phase", "verify", "--context", oldCapture, "--stdin")
	require.Error(t, err)
	require.Contains(t, out, "409")
	require.Equal(t, unchanged.ContractVersion, f.show(task.ID).ContractVersion)
	require.Equal(t, types.StatusOpen, f.show(task.ID).Status)
	f.session = "durable-worker-after"
	after, newCapture := f.claim(task.ID, "after")
	require.Equal(t, int64(2), after.Governing.Reference.Revision)
	require.Equal(t, int64(2), after.Governing.Context[0].Reference.Revision)
	require.Greater(t, after.ContractVersion, before.ContractVersion)
	for _, phase := range []string{"build", "review", "verify"} {
		require.Equal(t, after, f.evidence(task.ID, newCapture, phase).Expected)
	}
	f.cli("close", task.ID, "--context", newCapture, "--reason", "Verified both adopted revisions")
	require.Equal(t, types.StatusClosed, f.show(task.ID).Status)
	var history []storage.ExecutionEvidence
	evidenceURL := "/api/v1/projects/" + f.project + "/issues/" + task.ID + "/execution-evidence"
	decodeDurable(t, string(durableAPI(t, "GET", evidenceURL, "", nil, 200)), &history)
	require.Len(t, history, 4)
	require.Contains(t, history, originalEvidence)
	// Source paths were deleted after upload. Restart the owned server, then read
	// exact old bytes/digest, old discussion, evidence and archive history.
	f.stop()
	f.start(arcBinary)
	var shown struct {
		Plan     types.Plan                    `json:"plan"`
		Revision types.PlanRevisionWithContent `json:"revision"`
	}
	decodeDurable(t, f.cli("plan", "show", meta.Plan.ID, "--revision", "1", "--json"), &shown)
	require.Equal(t, metaBytes, shown.Revision.Content)
	require.Equal(t, meta.Revision.ContentSHA256, shown.Revision.ContentSHA256)
	require.Equal(t, "approved", shown.Revision.ReviewStatus)
	published, err := filepath.Glob(filepath.Join(f.root, f.project, meta.Plan.ID, "1-*.md"))
	require.NoError(t, err)
	require.Len(t, published, 1)
	retainedBytes, err := os.ReadFile(published[0])
	require.NoError(t, err)
	require.Equal(t, metaBytes, string(retainedBytes))
	require.NoDirExists(t, filepath.Join(f.dir, "drafts"), "server must not publish in client plans.dir")
	destination := filepath.Join(f.dir, "exact.md")
	f.cli("plan", "export", meta.Plan.ID, "--revision", "1", "--output", destination)
	bytes, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, metaBytes, string(bytes))
	f.failure("plan", "export", meta.Plan.ID, "--revision", "2", "--output", destination)
	bytes, err = os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, metaBytes, string(bytes))
	f.cli("plan", "archive", meta.Plan.ID, "--expected-version", strconv.FormatInt(shown.Plan.Version, 10), "--json")
	decodeDurable(t, f.cli("plan", "show", meta.Plan.ID, "--revision", "1", "--json"), &shown)
	require.Equal(t, "archived", shown.Plan.Lifecycle)
	require.Equal(t, "approved", shown.Revision.ReviewStatus)
	require.Equal(t, metaBytes, shown.Revision.Content)
	var comments []types.PlanComment
	decodeDurable(t, f.cli("plan", "comments", meta.Plan.ID, "--revision", "2", "--json"), &comments)
	require.Len(t, comments, 1)
	require.Equal(t, comment.ID, comments[0].ID)
	require.Equal(t, comment.Revision, comments[0].Revision)
	require.Equal(t, comment.LineNumber, comments[0].LineNumber)
	var reread []storage.ExecutionEvidence
	decodeDurable(t, string(durableAPI(t, "GET", evidenceURL, "", nil, 200)), &reread)
	require.Equal(t, history, reread)
	_, epicCapture := f.claim(epic.ID, "epic-completion")
	f.evidence(epic.ID, epicCapture, "verify")
	f.cli("close", epic.ID, "--context", epicCapture, "--reason", "Children verified")
	f.cli("plan", "restore", meta.Plan.ID, "--expected-version", strconv.FormatInt(shown.Plan.Version, 10), "--json")
	decodeDurable(t, f.cli("plan", "show", meta.Plan.ID, "--revision", "1", "--json"), &shown)
	require.Equal(t, "approved", shown.Revision.ReviewStatus)
	t.Logf("project=%s meta=%s@1/%s@2 tactical=%s@1/@2 original_sha256=%s old_evidence=%s adoption=%s", f.project, meta.Plan.ID, meta.Plan.ID, tactical.Plan.ID, meta.Revision.ContentSHA256, originalEvidence.ID, adoption.ID)
}

// A lost SAVE response must replay its original result even after another save,
// adoption and archive change all the mutable state checked for new requests.
func TestDurableSaveReplayAfterLaterSaveAdoptionAndArchive(t *testing.T) {
	f := newDurableFixture(t)
	created := f.createPlan("replay-design", "# Revision one\n")
	originalBytes := "\ufeff# Saved revision two\r\nRetain these exact bytes.\n"
	originalFile := f.file("saved-two.md", originalBytes)
	originalArgs := []string{
		"plan", "update", created.Plan.ID, originalFile,
		"--expected-revision", "1", "--idempotency-key", "original-save", "--json",
	}
	var saved storage.PlanWriteResult
	decodeDurable(t, f.cli(originalArgs...), &saved)
	require.False(t, saved.Replay)
	require.Equal(t, int64(2), saved.Revision.Revision)
	require.Equal(t, originalBytes, saved.Revision.Content)
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(originalBytes))), saved.Revision.ContentSHA256)
	laterFile := f.file("saved-three.md", "# Later revision three\n")
	var later storage.PlanWriteResult
	decodeDurable(t, f.cli("plan", "update", created.Plan.ID, laterFile,
		"--expected-revision", "2", "--idempotency-key", "later-save", "--json"), &later)
	require.Equal(t, int64(3), later.Revision.Revision)
	f.review(created.Plan.ID, 3)
	milestone := f.issue("Replay milestone", "milestone", "")
	proposal := f.proposal(milestone.ID, created.Plan.ID, 3)
	f.apply(milestone.ID, created.Plan.ID, 3, proposal.Request, "adopt-later-save")
	var current struct {
		Plan types.Plan `json:"plan"`
	}
	decodeDurable(t, f.cli("plan", "show", created.Plan.ID, "--revision", "3", "--json"), &current)
	f.cli("plan", "archive", created.Plan.ID, "--expected-version", strconv.FormatInt(current.Plan.Version, 10), "--json")
	decodeDurable(t, f.cli("plan", "show", created.Plan.ID, "--revision", "3", "--json"), &current)
	require.Equal(t, int64(3), current.Plan.HeadRevision)
	require.Equal(t, "archived", current.Plan.Lifecycle)
	pinBefore := f.show(milestone.ID)
	require.Equal(t, &types.PlanReference{PlanID: created.Plan.ID, Revision: 3}, pinBefore.GoverningPlan)
	var historyBefore []types.PlanRevision
	decodeDurable(t, f.cli("plan", "history", created.Plan.ID, "--json"), &historyBefore)
	require.Len(t, historyBefore, 3)
	var replay storage.PlanWriteResult
	decodeDurable(t, f.cli(originalArgs...), &replay)
	require.True(t, replay.Replay)
	replay.Replay = false
	require.Equal(t, saved, replay, "SAVE replay must return the original revision, bytes, digest and metadata")
	var after struct {
		Plan types.Plan `json:"plan"`
	}
	decodeDurable(t, f.cli("plan", "show", created.Plan.ID, "--revision", "3", "--json"), &after)
	require.Equal(t, current.Plan, after.Plan, "SAVE replay must not advance the current head or change lifecycle")
	pinAfter := f.show(milestone.ID)
	require.Equal(t, pinBefore.GoverningPlan, pinAfter.GoverningPlan)
	require.Equal(t, pinBefore.ContractVersion, pinAfter.ContractVersion)
	var historyAfter []types.PlanRevision
	decodeDurable(t, f.cli("plan", "history", created.Plan.ID, "--json"), &historyAfter)
	require.Equal(t, historyBefore, historyAfter, "SAVE replay must not duplicate a revision")
	t.Logf("SAVE key original-save replays %s@2 SHA256=%s after SAVE/adoption@3/archive; head/pin/history unchanged",
		created.Plan.ID, saved.Revision.ContentSHA256)
}
