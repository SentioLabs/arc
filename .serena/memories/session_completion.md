# Landing the plane (session completion)

When ending a work session, complete all steps:
1. File issues for remaining work.
2. Run quality gates if code changed (tests/linters/builds).
3. Update issue status (close finished, update in-progress).
4. Commit and push:
   - `git add .`
   - `git commit -m "description of changes"`
   - `git push`
   - `git status` must show "up to date with origin"
5. Clean up: clear stashes, prune remote branches.
6. Verify all changes committed and pushed.
7. Hand off context for next session.

Critical rules:
- Work is not complete until `git push` succeeds.
- Never stop before pushing.
- Do not say "ready to push"; you must push.
- If push fails, resolve and retry until it succeeds.
