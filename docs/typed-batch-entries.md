# Typed batch Ledger Entries — design notes

Why the typed-payload codegen works the way it does. For how to *use* it, see the
README; for how to change it, see `CONTRIBUTING.md`.

## The problem

`addLedgerEntries` commits a batch of Ledger Entries atomically. It takes one list
of one input type — `AddLedgerEntryInput{entry, ik}` — and `LedgerEntryInput.parameters`
is an opaque `JSON` scalar. GraphQL cannot express the parameter types of an
individual entry inside that list, so callers are left building untyped maps and
finding their mistakes at runtime.

The Fragment CLI already generates one `addLedgerEntry` mutation per entry type,
and those operations carry both missing facts: the entry type as a string literal,
and each parameter bound to a typed operation variable. The codegen reads them back
out.

| Path | Role |
| --- | --- |
| `internal/typedentries/derive.go` | reads operations, produces a payload model per entry type |
| `internal/typedentries/emit.go` | renders those models as the `typed_payloads` package |
| `main.go` (`addTypedPayloads`) | second pass after genqlient, in the same CLI |
| `batch/batch.go` | `Entry`, and the ordered, omitting JSON writer |
| `queries/batch.go` | `AddTypedLedgerEntries`, `RawEntry` |
| `internal/generated/` | committed generated output, and the tests over it |

## Decisions worth knowing

### Payloads go in a nested `typed_payloads` package

Not organisational. Parameters must be supplied **by name**: Go permits unkeyed
struct literals, and reordering two same-typed parameters there swaps their values
with no compile error and no runtime complaint. A leading `_ struct{}` field
prevents that — but only from *outside* the declaring package, since Go allows
positional assignment to unexported fields within it.

So the guarantee depends entirely on the payloads not living in the same package as
the code that constructs them. Emitting them into a package of their own is what
makes it hold by construction rather than by convention — a customer generating with
`--package main` and writing their calls in `main` would otherwise get no protection
at all. `keyed_literal_test.go` compiles a program in a separate package to prove it,
and would pass vacuously if written any other way.

### Unset fields are omitted, never sent as `null`

`encoding/json`'s `omitempty` cannot express this: it also drops `""`, `0` and
`false`, which are values a caller chose. So `batch.Object` builds JSON field by
field, skipping what was never set.

This is not tidiness. A `null` in the wrong place changes meaning — see the Ledger
reference note below.

### The common-field set is fixed, not derived from the operation

Every payload carries `Ik`, `LedgerIk`, `Posted`, `Description`, `Tags`, `Groups`
and `Conditions`, whether or not its source operation binds them. `lines` is never
exposed, since it cannot be combined with an entry that has a type.

Deriving the set from the operation is tempting and wrong. An operation binds only
the fields the CLI chose to expose, and that choice has already changed between CLI
versions — one generation binds `tags`, `groups` and `conditions`, another binds
`typeVersion` instead, and none bind `description`. A payload travels as an
`AddLedgerEntryInput`, so what the operation binds places no limit on what the
payload may carry; deriving it would invent a restriction the API does not have and
move a payload's surface whenever the CLI changed.

It also pays off on a real CLI quirk. For a Schema parameter named `posted`, the
CLI drops the colliding name when building the variable list but not when building
the `parameters` block, so the parameter binds to the predefined `$posted` that
already feeds `entry.posted` — one variable in two wire positions, which a caller
cannot set independently. Because `Posted` exists regardless, the generator splits
them back apart into `Posted` and `Posted2`. See `testdata/edge.graphql`.

### A payload's name always carries its version

`OrderPlacedV1Entry`, never `OrderPlacedEntry`. A name depends only on its own
identity — `(type, typeVersion)` — and never on which other operations were in the
input. Suffixing only on collision would mean that adding version 2 renames version
1, turning a purely additive Schema change into a breaking one for every existing
call site.

For the same reason a payload's identity is the pair, not the type alone: the same
type at two versions has different parameter sets, and collapsing them drops one and
posts the wrong version.

### Escaping is local; wire names are verbatim

A parameter's Go field name may be changed to be a legal, unique identifier. Its
wire name never is. Where escaping renames something the caller did not choose, the
generator warns.

### Non-scalar parameters fall back to `json.RawMessage`

A parameter whose Schema type is an enum or input object is typed as raw JSON, with
a warning naming the type.

To see why, it helps to be precise about what a codegen run produces. Given
`--output gen/client.go --package gen`, it writes two packages:

```
gen/client.go                         package gen             genqlient's client
gen/typed_payloads/typed_payloads.go  package typed_payloads  the payloads
```

genqlient emits an enum or input object into `gen`, the sibling package — and
`typed_payloads` does not import it. The SDK's own `queries` copy is not a
substitute either: genqlient only emits types its own operations reach, so
`queries.EntryGroupMatchInput` does not exist, and emitting it was a compile error.

The payloads *could* import that sibling. It is in the same module, there is no
cycle, and the import path is derivable by walking up to the enclosing `go.mod` —
the generator knows the package's name from `--package` but not its path, since
nothing on the command line gives it one. It does not, in exchange for avoiding a
filesystem-dependent path that fails when `--output` sits outside a module.

In practice the CLI types every Schema parameter as `String!`, so this only affects
hand-written operations.

### `Tags`, `Groups` and `Conditions` use the SDK's input types

They are `[]queries.LedgerEntryTagInput` and friends, from
`github.com/fragment-dev/fragment-go/v4/queries` — not from the sibling package
genqlient wrote. This one is forced rather than chosen: those types are absent from
that sibling unless the customer's own operations happen to reference them, and the
common-field set does not depend on what their operations reference.

A customer generating with `--package fragment` therefore has both
`fragment.LedgerEntryTagInput` and payloads wanting `queries.LedgerEntryTagInput`:
identical in shape, distinct Go types. The generated package doc says so, because
the compile error does not explain itself.

### `RawEntry` omits its unset fields too

Found against a live API, and not reproducible offline. Marshalling
`AddLedgerEntryInput` directly encodes a `LedgerMatchInput` with only `ik` set as
`{"id":null,"ik":"L"}`, and the API resolves that as a **different Ledger** from the
`{"ik":"L"}` a typed payload sends:

```
invalid_input_provided: Bad Request: entries[1] targets a different Ledger;
all entries in a batch must target the same Ledger.
```

Both entries named the same Ledger. So a raw entry that kept its nulls could not
share a batch with a typed one at all. `RawEntry` now omits unset fields; values
inside `parameters` pass through untouched, since those are the caller's own JSON.

Nothing is given up. A nil `*string` in the generated input types means both "unset"
and "null" with no way to distinguish them, so there was never an explicit null a Go
caller could have intended.

### Warnings, not errors, for a stale `.graphql`

Two operations claiming the same `(type, typeVersion)` but declaring different
parameters means the `.graphql` is stale relative to the Schema. The first wins and
the generator warns, rather than failing a customer's build over it.

An operation whose `typeVersion` is present but not an integer literal is skipped
with a warning instead: its identity is undefined, and defaulting to 1 would post
version 1 for an operation that accepts any version.

## Limits the SDK cannot enforce

A batch is capped at **30 Ledger Lines in total**, not 30 entries — so 15 entries of
a 2-line type, or 5 of a 6-line type. Exceeding it is a `BadRequestError`, and since
the batch is atomic, nothing commits.

There is no check to write, even at runtime: an entry type's line count lives in the
Schema and never reaches the operations the generator reads. Callers have to size
chunks by lines.

## What the tests cover, and why each exists

| Test | Why it exists |
| --- | --- |
| `internal/generated/wire_test.go` | `TestWireFormat` is the worked example — one batch, the complete JSON it produces. The canonical reference for the format. |
| `internal/generated/compile_test.go` | Builds generated source for hostile operation sets. Every codegen defect found so far produced source `gofmt` accepted and the compiler rejected, so nothing short of building it would catch them. |
| `internal/generated/keyed_literal_test.go` | Proves unkeyed literals fail, from a separate package. |
| `internal/typedentries/golden_test.go` | Snapshot: generated identifiers cannot change without a reviewable diff. Callers write those names by hand. |
| `internal/typedentries/derive_test.go` | The derivation rules, case by case. |
| `queries/batch_test.go` | `RawEntry` semantics, all of it discovered live. |
| `batch/batch_test.go` | The JSON writer: insertion order, omission, error propagation. |
| `main_test.go` | The CLI wiring, reading the real binding table rather than a copy. |
| `internal/livetest/` | A real API. See `CONTRIBUTING.md`. |

Two habits are worth keeping when changing the generator, both learned the hard way:

- **Build the generated output, do not just inspect it.** A test asserting on
  derived values or emitted text cannot see a missing import, a field colliding with
  a method, or a helper called with the wrong type.
- **Feed tests the real inputs.** Several defects were masked by a test supplying
  something the CLI does not — a scalar table containing `String`, or an expected
  value that encoded the bug. `main_test.go` reads the binding table from
  `newCodegenConfig`, and the built-in scalars are deliberately absent from every
  test table so the derivation has to resolve them itself.
