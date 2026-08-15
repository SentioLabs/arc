# Arc Issue Tracker — Makefile
# ============================================================================
# THIN WRAPPER — all targets live in Taskfile.yml. Nothing is declared here.
# `make <target> ...` forwards verbatim to `task <target> ...`.
# Run `task --list` (or plain `make`) to see available tasks.
#
# Do not add targets to this file. Add tasks to Taskfile.yml or taskfiles/*.yml.
# ============================================================================

# Resolve the task binary: PATH first, then mise, else fail with a hint.
TASK ?=
ifeq ($(strip $(TASK)),)
  TASK := $(shell command -v task 2>/dev/null)
  ifeq ($(strip $(TASK)),)
    ifneq ($(shell command -v mise 2>/dev/null),)
      TASK := mise exec -- task
    endif
  endif
endif
ifeq ($(strip $(TASK)),)
$(error go-task not found. Run `mise install`, or see https://taskfile.dev/installation/)
endif

.DEFAULT_GOAL := _arc_forward

_arc_forward: FORCE
	@$(TASK)

# One `task` invocation carrying every goal, so go-task can dedupe shared
# prerequisites. Only the first goal fires; the rest are satisfied silently.
%: FORCE
	@[ "$@" != "$(firstword $(MAKECMDGOALS))" ] || $(TASK) $(MAKECMDGOALS)

FORCE: ;
Makefile: ;      # stop make from trying to remake this file via the % rule
.PHONY: FORCE
