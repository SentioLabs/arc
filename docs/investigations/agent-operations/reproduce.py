#!/usr/bin/env python3
"""Reproduce Arc agent-operation semantics using only a disposable database."""
import concurrent.futures
import json
import pathlib
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request


def investigate(binary, fixture, url):
    def request(method, path, data=None, actor="fixture"):
        payload = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(url + path, data=payload, method=method,
                                     headers={"Content-Type": "application/json", "X-Actor": actor})
        try:
            with urllib.request.urlopen(req, timeout=10) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            with error:
                return error.code, json.load(error)

    def ok(method, path, data=None, actor="fixture"):
        status, result = request(method, path, data, actor)
        assert 200 <= status < 300, (status, result)
        return result

    project = ok("POST", "/api/v1/projects", {"name": "Operation investigation", "prefix": "probe"})
    base = "/api/v1/projects/" + project["id"]
    ok("POST", base + "/workspaces", {"path": fixture})

    def issue(title):
        return ok("POST", base + "/issues", {"title": title, "description": "D", "issue_type": "task"})

    def path(item):
        return base + "/issues/" + item["id"]

    results = {}
    item = issue("Stale note versus specification edit")
    stale = ok("GET", path(item))["description"]
    ok("PUT", path(item), {"description": stale + "+spec-B"}, "writer-B")
    intended = stale + "+note-A"
    ok("PUT", path(item), {"description": intended}, "writer-A")
    final = ok("GET", path(item))["description"]
    assert final == intended and "spec-B" not in final
    results["stale_note_overwrites_spec"] = {"readback_check_passes": final == intended, "description": final}

    # The original response could have been lost; repeating the same successful
    # update later is safe only if no intervening writer changed the description.
    ok("PUT", path(item), {"description": intended + "+spec-C"}, "writer-C")
    ok("PUT", path(item), {"description": intended}, "writer-A-retry")
    final = ok("GET", path(item))["description"]
    assert final == intended and "spec-C" not in final
    results["lost_update_response_retry"] = {"description": final, "intervening_spec_edit_lost": True}

    item = issue("Two stale note appenders")
    reads = threading.Barrier(2)
    a_saved = threading.Event()

    def append_note(writer):
        old = ok("GET", path(item))["description"]
        reads.wait(timeout=10)
        if writer == "B":
            assert a_saved.wait(timeout=10)
        ok("PUT", path(item), {"description": old + "+note-" + writer}, writer)
        if writer == "A":
            a_saved.set()

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        list(pool.map(append_note, ("A", "B")))
    final = ok("GET", path(item))["description"]
    assert final == "D+note-B"
    results["two_note_appenders"] = {"description": final, "note_A_lost": True}

    item = issue("Comments plus concurrent specification edit")
    start = threading.Barrier(3)

    def comment_or_edit(writer):
        start.wait(timeout=10)
        if writer == "spec":
            return ok("PUT", path(item), {"description": "D+spec-B"}, writer)
        return ok("POST", path(item) + "/comments", {"text": "note-" + writer}, writer)

    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        list(pool.map(comment_or_edit, ("A", "B", "spec")))
    details = ok("GET", path(item) + "?details=true")
    assert details["description"] == "D+spec-B"
    assert sorted(c["text"] for c in details["comments"]) == ["note-A", "note-B"]
    results["comments_preserve_spec_and_notes"] = {
        "description": details["description"], "comments": sorted(c["text"] for c in details["comments"])}

    # Discard the first successful response, then retry the identical append.
    ok("POST", path(item) + "/comments", {"text": "retry-note"})
    ok("POST", path(item) + "/comments", {"text": "retry-note"})
    comments = ok("GET", path(item) + "/comments")
    duplicates = sum(c["text"] == "retry-note" for c in comments)
    assert duplicates == 2
    results["lost_response_retry"] = {"identical_comment_count": duplicates}

    parent = ok("POST", base + "/issues", {"title": "Parent design", "issue_type": "epic"})
    child = ok("POST", base + "/issues", {"title": "Unlabeled child", "parent_id": parent["id"]})
    details = ok("GET", path(child) + "?details=true")
    assert not details.get("parent_id") and "labels" not in details
    assert any(d["type"] == "parent-child" and d["depends_on_id"] == parent["id"]
               for d in details["dependencies"])
    results["handoff_detail_shape"] = {"parent_id_present": "parent_id" in details,
                                        "labels_present": "labels" in details,
                                        "parent_in_dependencies": True}

    sessions = ["registered-session-A", "registered-session-B"]
    for session in sessions:
        ok("POST", base + "/ai/sessions", {"id": session, "cwd": fixture})
    item = issue("Competing explicit claims")
    start = threading.Barrier(2)

    def claim(session):
        observed = ok("GET", path(item))
        assert observed["status"] == "open" and not observed.get("ai_session_id")
        start.wait(timeout=10)
        command = [binary, "--config", fixture + "/config.toml", "--server", url,
                   "update", item["id"], "--take", "--session-id", session]
        result = subprocess.run(command, cwd=fixture, text=True, capture_output=True, check=False)
        assert result.returncode == 0, result.stderr
        return {"session": session, "exit_code": result.returncode, "stdout": result.stdout.strip()}

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        responses = list(pool.map(claim, sessions))
    final = ok("GET", path(item))
    assert final["ai_session_id"] in sessions and final["status"] == "in_progress"
    results["competing_claims"] = {
        "responses": responses, "owner": final["ai_session_id"], "status": final["status"],
        "implementation_workers_executed": False}

    item = issue("Injected interruption after first field write")
    # Count only updates to this disposable issue and reject its second field write,
    # regardless of Go's map iteration order. No production DB is opened here.
    with sqlite3.connect(fixture + "/data.db") as db:
        db.executescript("""
            CREATE TABLE fixture_writes (n INTEGER);
            CREATE TRIGGER fixture_abort_second BEFORE UPDATE OF status, ai_session_id ON issues
            WHEN OLD.id = '%s' AND (SELECT COUNT(*) FROM fixture_writes) > 0
            BEGIN SELECT RAISE(ABORT, 'fixture: second field write rejected'); END;
            CREATE TRIGGER fixture_record_first AFTER UPDATE OF status, ai_session_id ON issues
            WHEN OLD.id = '%s'
            BEGIN INSERT INTO fixture_writes VALUES (1); END;
        """ % (item["id"], item["id"]))
    status, response = request("PUT", path(item), {"status": "in_progress", "ai_session_id": sessions[0]})
    assert status == 500, (status, response)
    final = ok("GET", path(item))
    assert (final.get("ai_session_id") == sessions[0]) != (final["status"] == "in_progress")
    results["interrupted_multifield_update"] = {
        "http_status": status, "response": response,
        "owner": final.get("ai_session_id"), "status": final["status"]}
    return results


def main():
    binary = str(pathlib.Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory(prefix="arc-operations-") as fixture:
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            port = sock.getsockname()[1]
        url = "http://127.0.0.1:" + str(port)
        with open(fixture + "/server.log", "w+") as log:
            server = subprocess.Popen([binary, "--config", fixture + "/config.toml", "server", "start",
                                       "--foreground", "--port", str(port), "--db", fixture + "/data.db"],
                                      cwd=fixture, stdout=log, stderr=log)
            try:
                for attempt in range(100):
                    try:
                        with urllib.request.urlopen(url + "/health", timeout=1):
                            break
                    except OSError:
                        if server.poll() is not None:
                            log.seek(0)
                            raise RuntimeError(log.read())
                        time.sleep(.05)
                else:
                    raise RuntimeError("temporary server startup timed out")
                print(json.dumps(investigate(binary, fixture, url), indent=2))
            finally:
                server.send_signal(signal.SIGTERM)
                server.wait(timeout=15)


if __name__ == "__main__":
    main()
