# Arc Plugin Eval Suite

Trigger and quality evals for the arc Claude Code plugin.

## Structure

```
evals/
  triggers/
    skills.json       # Trigger evals for workflow skills (brainstorm, plan, implement, etc.)
    commands.json      # Trigger evals for slash commands (/arc:ready, /arc:create, etc.)
    agents.json        # Trigger evals for agent dispatch (issue-tracker, implementer, etc.)
  quality/
    brainstorm.json    # Quality evals for brainstorm skill output (5 evals)
    plan.json          # Quality evals for plan skill output (5 evals)
    implement.json     # Quality evals for implement skill output (9 evals)
    debug.json         # Quality evals for debug skill output (5 evals)
    verify.json        # Quality evals for verify skill output (5 evals)
    review.json        # Quality evals for review skill output (7 evals)
    finish.json        # Quality evals for finish skill output (6 evals)
```

## Running Evals

### Via skill-creator (recommended)

```
/skill-creator Eval my arc brainstorm skill
/skill-creator Benchmark my arc skills across 5 runs
```

### Manual trigger eval

Each trigger eval file contains queries with `should_trigger` booleans.
Pass rate = correct activations / total queries.

**Target**: 90%+ pass rate on trigger evals, 80%+ on quality evals.

## Writing Good Eval Queries

- **Be concrete**: Include project names, issue IDs, file paths, error messages
- **Add backstory**: "I've been working on the auth module and just finished the JWT implementation"
- **Test boundaries**: Queries that are close to triggering but shouldn't (e.g., "plan my weekend" should NOT trigger the plan skill)
- **Mix positive/negative**: ~60% should_trigger=true, ~40% false
- **Avoid abstractions**: "Do the thing" is bad. "Create a bug for the login timeout we discussed" is good.
