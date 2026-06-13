# Domain context

Shared vocabulary for infracollect. Names here are the load-bearing concepts a
contributor (or an architecture review) should reuse verbatim rather than
reinventing.

## Terms

**Job template**
An HCL file describing a collection job: its `collector` blocks, `step` blocks,
and `output` block. Parsed into a `JobTemplate`.

**Collector**
A configured instance of a data source provider (a Terraform provider, an HTTP
endpoint). Has a lifecycle (`Start`/`Close`) and is referenced by steps via
`collector.<type>.<id>`.

**Step**
A single data-collection operation that resolves to a `Result`. Referenced by
later steps via `step.<type>.<id>`. A step with `for_each` becomes a
**collection** node that fans out over its elements.

**Pipeline**
The resolvable DAG built from a job template — nodes (collectors, steps,
collections) plus per-node metadata (`Body`, `Refs`, `ForEach`,
`CollectorAddr`).

**Scope** _(runner internal)_
The lexical scope that later nodes resolve `step.*` and `collector.*`
references against during execution. Owns the incremental cty mirrors of those
two namespaces and is the **only** place that builds a per-node
`hcl.EvalContext`. Lives in `internal/runner/scope.go`, separate from the
`Runner`, which keeps the live collectors and the raw results. Collector
entries are sentinels (`cty.EmptyObjectVal`) — they exist so a `collector =
collector.<type>.<id>` binding type-checks during step-body decode; the binding
is never evaluated, it is walked directly by `resolveStepCollector`.

**Result**
What a step produces: `{ id, data, meta }`. Exposed to downstream nodes through
the scope as `step.<type>.<id>.data` / `.meta`, and handed to the result writer
at the end of a run.

**Result writer** _(engine)_
The single object the Runner feeds results to once execution finishes. Owns the
file-naming convention (`<id>.<ext>` for data, `<id>.meta.<ext>` for metadata),
the rule that metadata is only written when present, and the close ordering.
Lives in `internal/engine/resultwriter.go` as a concrete `ResultWriter` over the
`Encoder` and `Sink` seams (an `ArchiveSink` may stand in for the sink). The
Runner decides *which* results and in *what order*; the writer owns *how* each
becomes bytes on a sink.
