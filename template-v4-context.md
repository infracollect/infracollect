# Template v4 — Recovered Context

Reconstructed from the `template-v3` session subagent logs (main transcript
`93e17b9d-...jsonl` was deleted, only `subagents/agent-a86fd19.jsonl` survived),
plus the on-disk state of branch `template-v4`.

Sibling docs:

- [`template-DESIGN.md`](./template-DESIGN.md) — the redesign plan (locked in)
- [`cel-templating-engine.md`](./cel-templating-engine.md) — Kro CEL engine study / prior art

## 1. Why this rewrite exists

The old templating system expanded all `${...}` references **before the pipeline
was built**, using Go's `os.Expand`. That made four things impossible:

1. Step-output references (`${steps.fetch.data.endpoint}` in a later step)
2. Dynamic `for_each` (iterate over data a previous step returned)
3. Conditional step execution based on previous results
4. Any late-bound configuration — every value had to be known at parse time

Current flow:

```text
Parse YAML → Expand ALL templates → Create Pipeline → Execute Steps
                 ↑
             Too early — step results don't exist yet.
```

Target flow:

```text
Parse YAML → Build DAG → Execute in order → Expand templates just before each node
                              ↓
                    Previous results available
                    for template expansion
```

## 2. The "too early" expansion point (old system)

The critical line is `cmd/infracollect/collect.go:103`:

```go
if err := runner.ExpandTemplates(&job, variables); err != nil {
    return fmt.Errorf("failed to expand templates: %w", err)
}
// ... runner.New and pipeline creation happen AFTER this
```

Execution timeline in the old system:

```text
cmd/infracollect/collect.go
  ├─ ParseCollectJob(jobFile)          // strings still raw
  ├─ BuildVariables(job, allowedEnv)   // JOB_NAME, JOB_DATE_*, --pass-env
  ├─ ExpandTemplates(&job, variables)  // ← EARLY RESOLUTION
  ├─ runner.New(...)                   // specs arrive already expanded
  └─ r.Run(ctx)
         └─ engine/pipeline.go: steps run sequentially, results accumulate
            into map[string]Result but are never fed back into templates
```

`BuildVariables` (old `internal/runner/pipeline.go:220-246`) produced only:

- `JOB_NAME`
- `JOB_DATE_ISO8601`
- `JOB_DATE_RFC3339`
- allowed env vars from `--pass-env`

No step outputs, no cross-step state. Ever.

## 3. Templateable fields (old inventory)

From `apis/v1/job.go`, fields carrying `template:""`:

| Owner                | Fields                                   |
| -------------------- | ---------------------------------------- |
| `HTTPCollector`      | `BaseURL`, `Headers`                     |
| `HTTPBasicAuth`      | `Username`, `Password`, `Encoded`        |
| `HTTPGetStep`        | `Headers`, `Params`                      |
| `ExecStep`           | `Program` (slice), `Env`                 |
| `StaticStep`         | `Value`                                  |
| `FilesystemSinkSpec` | `Prefix`                                 |
| `S3SinkSpec`         | `Bucket`, `Region`, `Endpoint`, `Prefix` |
| `S3Credentials`      | `AccessKeyID`, `SecretAccessKey`         |
| `ArchiveSpec`        | `Name`                                   |

Notably **not** templateable in the old system: `ExecStep.Input`, `WorkingDir`,
`Timeout`, `Format`. Worth revisiting in v4.

`template:"-"` means "skip". Untagged string fields are not expanded even if
they contain `${...}`.

## 4. No step-dependency tracking in the old engine

Old `internal/engine/pipeline.go` runs steps in YAML source order:

```go
for _, entry := range p.steps {
    result, err := entry.Step.Resolve(ctx)
    results[entry.ID] = result
}
```

No DAG. No ordering beyond source order. Results accumulate into a
`map[string]Result` that is **only** used by the output sink — later steps
cannot see it.

`for_each` has the same early-binding disease: old
`internal/runner/pipeline.go:31-50` iterates `collectorSpec.ForEach.All()`
at pipeline-creation time, so you can't iterate over something a previous step
fetched.

## 5. The v4 design (from `template-DESIGN.md`)

**Breaking change**: all references become namespaced.

| Namespace | Meaning               | Example                        |
| --------- | --------------------- | ------------------------------ |
| `env.`    | environment variables | `${env.API_KEY}`               |
| `job.`    | job metadata          | `${job.name}`, `${job.date}`   |
| `steps.`  | step outputs          | `${steps.fetch.data.url}`      |
| `each.`   | for_each iteration    | `${each.key}`, `${each.value}` |

The parser **rejects unscoped `${FOO}`** with a clear error. Migration:

- `${API_KEY}` → `${env.API_KEY}`
- `${JOB_NAME}` → `${job.name}`
- `${JOB_DATE_RFC3339}` → `${job.date}`
- `${JOB_DATE_ISO8601}` → removed (use `${job.date}`)

Internal type renames applied in the same pass:

- `RefTypeBuiltin` dropped; now `RefTypeEnv` / `RefTypeJob` / `RefTypeStep` / `RefTypeEach`
- `Reference.ID` → `Reference.Key`
- `VariableContext.Builtins map[string]string` → `VariableContext.Job JobContext`

Evaluator: **`github.com/google/cel-go/cel`**. Rich reference examples live in
`template-DESIGN.md:87-103` (including array indexing like
`${steps.fetch.data.items[0]}` and nested paths).

The five architectural moves the design commits to:

1. Move expansion **after** step creation — templates become placeholders.
2. Track step dependencies in a DAG.
3. Evaluate lazily at step-execution time.
4. Thread previous `Result`s into a template context.
5. Namespaced syntax (above).

Goals: explicit dependencies, fail-fast cycle/missing-ref detection at parse
time, small testable components.

## 6. Current `template-v4` working tree state

Branch `template-v4`, committed as `be90d12 wip` (not in `main`), plus
uncommitted changes on top. Pulls `cel-go` into `go.mod`/`go.sum`.

### `internal/runner/dag.go` (committed + ~+176 uncommitted lines)

Generic DAG in the `runner` package:

```go
type NodeType int
const (
    NodeTypeCollector NodeType = iota
    NodeTypeStep
)

type Node struct { Kind NodeType; ID string }
func (n Node) Key() string        // "collector:foo" / "step:bar"

type DirectedAcyclicGraph struct {
    nodes map[string]Node
    edges map[string][]string
    order []Node                   // cached topo sort
}

func (g *DirectedAcyclicGraph) AddNode(node Node) error
func (g *DirectedAcyclicGraph) AddEdge(from, to Node) error   // cycle-safe
func (g *DirectedAcyclicGraph) WouldCreateCycle(from, to Node) (bool, error)
func (g *DirectedAcyclicGraph) TopologicalSort() ([]Node, error)  // Kahn's
```

- `AddEdge` calls `WouldCreateCycle` → refuses to create a cycle.
- `canReach` BFS is the cycle probe.
- `kahnSort` builds in-degree map, picks zero-in-degree nodes in sorted order
  (`insertSorted`) so the topo order is deterministic.
- Any mutation invalidates the cached `order`.
- **Not** concurrency-safe; documented with a comment.

Untracked `internal/runner/dag_test.go` (~119 lines) covers the above.

### `internal/runner/template.go` (uncommitted, ~+493/−180)

Rewritten around CEL. Key types:

```go
type TemplateExpression struct {
    Expression string
    References []string
    Program    cel.Program
}

type TemplateField struct {
    Path                 string
    Expressions          []TemplateExpression
    StandaloneExpression bool
}
```

Walker (`parseTemplate` → `parseObject` / `parseArray` / `parseString`)
recurses a `map[string]any`, builds a flat `[]TemplateField`, each with a
JSON-pointer-like `Path` (`foo.bar[0].baz`). Standalone expressions
(whole-value `${...}`) are flagged so their result type can be preserved
instead of coerced to string.

`extractExpressions` (referenced but not yet inspected in this doc) parses the
`${...}` delimiters; see `celExprStart`/`celExprEnd` constants at the top of
the file.

### Runner → engine split (in progress)

- `wip` commit **removed** 91 lines from `internal/engine/pipeline.go`.
- A fresh **untracked** `internal/engine/pipeline.go` exists — looks like
  the split started but isn't finished.
- `internal/runner/pipeline.go` gained a small diff on top (~+38/−? in `wip`,
  plus uncommitted tweaks).
- `internal/runner/run.go` got +72/−? in `wip`.

### Backlog

`BACKLOG.md:108` — **`[~] Advanced DAG engine`** is in-progress. That's this
work. `BACKLOG.md:152` ("More template pattern / Evaluate goexpr or
gotemplate") is the P3 item that kicked it off.

## 7. What's missing / what's next

What we cannot recover:

- The main `93e17b9d-...jsonl` transcript is gone. Conversation-level
  decisions like "we picked cel-go over `expr` because X" or "we rejected
  approach Y" are lost. Only the design doc's outcome survives.

What the design calls for that isn't obviously done yet (needs verification):

- [ ] Wiring: `runner.Run` must build the DAG from parsed specs, topo-sort,
      and expand templates **per-node** using a context populated with
      prior results.
- [ ] `VariableContext.Job` struct (replacing the old `Builtins` map).
- [ ] Parser rejection of unscoped `${FOO}` with error message.
- [ ] `each.*` namespace plumbed through `for_each` nodes.
- [ ] `steps.*` references wired to `Result.Data` / `Result.Meta`.
- [ ] Cycle detection surfaced at parse time (DAG has the primitive; the
      builder has to use it).
- [ ] Full test coverage for the CEL walker (`dag_test.go` exists; no
      `template_test.go` rewrite yet).
- [ ] Finish the runner→engine split (new `internal/engine/pipeline.go` is
      still untracked).
- [ ] Migrate existing integrations (`http`, `terraform`, `exec`, sinks) to
      the new per-node expansion model — right now they expect pre-expanded
      specs.
- [ ] Revisit the `template:""` inventory: decide whether `ExecStep.Input`,
      `WorkingDir`, `Timeout`, `Format` should become templateable in v4.

## 8. Key files index

| File                                          | Role                                                                                     |
| --------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `cmd/infracollect/collect.go`                 | Old early-expansion site (`:103`); needs to stop calling `ExpandTemplates` pre-pipeline. |
| `internal/runner/template.go`                 | New CEL-based walker and expression model.                                               |
| `internal/runner/dag.go`                      | New DAG with Kahn sort and cycle detection.                                              |
| `internal/runner/dag_test.go`                 | DAG tests (untracked).                                                                   |
| `internal/runner/pipeline.go`                 | Pipeline builder — where the DAG should be assembled.                                    |
| `internal/runner/run.go`                      | Execution driver — where lazy expansion should happen.                                   |
| `internal/engine/pipeline.go`                 | Execution loop; old sequential version shrunk, new untracked version in progress.        |
| `apis/v1/job.go`                              | Source of `template:""` tags; will need `each.*` / `steps.*` awareness.                  |
| `internal/integrations/{http,terraform,exec}` | Consumers that currently expect pre-expanded specs.                                      |
| `template-DESIGN.md`                          | The plan.                                                                                |
| `cel-templating-engine.md`                    | Kro reference study.                                                                     |
| `BACKLOG.md:108`                              | `[~] Advanced DAG engine` tracking item.                                                 |
