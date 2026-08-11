# Typed batch Ledger Entries — conformance

How this SDK satisfies the shared specification in
[`fragment-dev/graphql-queries`](https://github.com/fragment-dev/graphql-queries), which
also governs `fragment-python`, `node-client`, and `fragment-ruby`.

Spec version implemented: **0.1.0**
Equivalence profile: **baseline** (semantic equivalence; §3.4)

Section numbers refer to `shared-spec/typed-batch-entries.md`.

## Where things live

| Path | Role |
| --- | --- |
| `internal/typedentries/derive.go` | §2.1–2.5 derivation from `.graphql` operations |
| `internal/typedentries/emit.go` | generates the `typed_payloads` package |
| `internal/typedentries/golden_test.go` | the §2.6 snapshot test, and `-update` |
| `main.go` (`addTypedPayloads`) | wires derivation into the codegen CLI |
| `batch/batch.go` | `Entry` interface and the ordered, omitting JSON writer |
| `queries/batch.go` | `AddTypedLedgerEntries`, `RawEntry` |
| `internal/conformance/` | the fixtures, the committed generated output, and the wire tests |
| `internal/conformance/compile_test.go` | type-checks generated output for hostile inputs |

The snapshot writer lives in `internal/typedentries` rather than next to the
fixtures on purpose: `internal/conformance` imports the generated payloads, so it
cannot compile while they are stale — which is exactly when they need rewriting.

## Section by section

| § | Requirement | How |
| --- | --- | --- |
| 2.1 | Recognition | `recognize` in `derive.go`. All six conditions; anything failing one returns `false` and is skipped without an error. |
| 2.1 | Own operations excluded | Falls out of the string-literal `type` condition: the SDK's `AddLedgerEntry` binds `type: $entryType`. Asserted by `TestSDKOperationsDeriveNothing`. |
| 2.1 | Name irrelevant | No name is consulted. `TestOperationNameIsIrrelevant`. |
| 2.2 | Identity is `(type, typeVersion)` | `Derive` keys on both. `TestDistinctVersionsAreDistinctPayloads`. |
| 2.2 | Dedup, first wins | Same key returns early. `TestUnpinnedAndVersionOneShareAnIdentity`. |
| 2.2 | Differing parameters | Warning, not an error — the spec makes erroring a MAY, and this usually means a stale `.graphql` rather than invalid input. `TestDuplicateIdentityWithDifferentParametersWarns`. |
| 2.3 | Non-variable parameters skipped | `paramsOf` skips anything that is not `ast.Variable`. `TestNonVariableParametersAreSkipped`. |
| 2.3 | Types from variable definitions | `op.VariableDefinitions.ForName`, never the field name. `TestParameterTypesComeFromVariableDefinitions`. |
| 2.3 | Untyped fallback | `Payload.Untyped` emits a `json.RawMessage` field. `TestParametersWithoutInlineObjectFallBackToUntyped`, `TestUntypedParametersFallback`. |
| 2.3 | Duplicate wire names | Dropped with a warning; two fields of one name would put the same key on the wire twice. `TestDuplicateWireNamesAreDropped`. |
| 2.3a | Common fields | `CommonFields` in `derive.go`, a fixed list, with `reservedFieldNames` derived from it so the two cannot drift. Not derived from the operation. `lines` deliberately absent. `TestCommonFieldsAreFixed`, `TestCommonFieldsAreEmittedRegardlessOfWhatTheOperationBinds`, `TestCommonFieldsDoNotDriftFromReservedNames`. |
| 2.4 | Source order | `paramsOf` appends in `Children` order; `batch.Object` preserves insertion order, which a Go map could not. `TestParameterOrderIsSourceOrder` in both packages. |
| 2.5 | Name carries version | `payloadName` always appends `V<n>`. `TestPayloadNamesCarryTheirVersion`. |
| 2.5 | Unpinned normalises to 1 | `typeVersionOf`. Applied to identity, name, and wire value alike. |
| 2.5 | Local escaping | `escapeFields`; first occurrence keeps the plain name, later ones are suffixed. Warns on rename. |
| 2.5 | Colliding names stay distinct | `TestCollidingFieldNamesStayDistinct`, plus `TestParametersCannotShadowCommonFields` for the `posted` case. Reserved names also cover the methods every payload declares, since a field cannot share a name with one. |
| 2.5 | Identifiers are always legal | `exportedIdent` sanitises to letters and digits. An entry type is a free-form Schema string, so an unsanitised one would reach gofmt and fail the run. `TestAwkwardEntryTypesProduceValidIdentifiers`. |
| 2.6 | Additive changes don't break callers | `TestNewOptionalParameterDoesNotRenameExistingFields`, `TestPayloadNamesCarryTheirVersion`. |
| 2.6 | Go must require keyed literals | Leading `_ struct{}` on every payload. `TestUnkeyedLiteralDoesNotCompile`, which builds from a separate package because the rule is cross-package only. |
| 2.6 | Snapshot test | `TestGeneratedPayloadsAreCurrent` diffs regenerated output against the committed files in `internal/conformance/f00N/`. |
| 3.1 | Shape and entry order | Generated `MarshalJSON`; `TestEntryOrderPreserved`. |
| 3.2 | Unset omitted, never null | `batch.SetOpt` / `batch.SetSlice`, dispatched on the Go type's shape rather than on required-ness, since a nullable list is a slice and not a pointer. `omitempty` is not used anywhere on this path. `TestUnsetIsOmittedNotNull`, `TestSetOptOmitsNilAndKeepsZeroValues`, `TestUntypedParametersOmittedWhenNil`. |
| 3.3 | Verbatim wire names | `Param.WireName` is never transformed; escaping only touches `FieldName`. Fixture `003`. |
| 3.4 | Baseline equivalence | Comparison is parsed-JSON equality in `assertJSONEqual`. |
| 3.5 | Mixing raw and typed | `queries.RawEntry`, which omits unset fields so that both kinds encode a Ledger the same way — see the note below. `TestMixingRawAndTypedEntries`, `queries/batch_test.go`, and `TestAddTypedLedgerEntriesMixedWithRaw` against a live API. |
| 3.6 | Everything accepted serialises | `AddTypedLedgerEntries` accepts only `batch.Entry`, whose sole method is the marshaller, so there is nothing acceptable that cannot serialise. |
| 4 | Batch semantics | Inherited from the API, and verified live: `TestAddTypedLedgerEntriesReportsIkReplay` and `TestAddTypedLedgerEntriesIsAtomic` in `internal/livetest`. `AddLedgerEntriesError.Errors` is surfaced per entry, each carrying its `ik`. |

## Deviations

### §5 — no runner over the shared fixtures

**What the spec asks:** each SDK implements a runner that generates from
`input.graphql`, builds the batch in `case.json`, serializes, and compares to
`expected.json`.

**What this SDK does:** the fixtures are copied into `internal/conformance/testdata/`
and each case is written out by hand in `wire_test.go`, against generated output that
is committed under `internal/conformance/f00N/`. Two local fixtures cover shapes the
shared six do not: `007-cli-output` is real Fragment CLI output, the only input shape
customers actually have, and `008-untyped-parameters` covers the §2.3 fallback.

**Why:** Go cannot do it in one process. The payloads are generated source and must be
compiled before anything can construct them, so a runner would need a build step
between reading `input.graphql` and building the batch. Resolving `("user-funds-account", 2)`
to a compiled Go type at runtime would additionally need reflection over field names,
which is exactly the mapping the generator already knows statically.

**Cost, stated plainly:** this SDK will *not* go red on its own when upstream changes a
fixture. The `.graphql`, `case.json`, and `expected.json` files under
`internal/conformance/testdata/` are copies, and re-syncing them is currently manual.
The `updateSDKQueries` workflow does not yet fetch `shared-spec/`.

### §2.3 — enum and input-object parameters are not typed precisely

**What the spec asks:** a parameter's type comes from the matching variable
definition.

**What this SDK does:** scalars are typed precisely. A parameter whose Schema type
is an enum or input object becomes `json.RawMessage`, with a warning naming the
type.

**Why:** genqlient emits such a type into the package it generated for *those*
operations, and the payloads do not import that package. Referencing the SDK's
`queries` copy is not an option: genqlient only emits types its own operations
reach, so `queries.EntryGroupMatchInput` does not exist, and emitting it was a
compile error.

To be precise about what blocks this, since it is a choice rather than a hard
limit: the payloads *could* import the customer's package. It is in the same module,
there is no cycle, and the import path is derivable by walking up to the enclosing
`go.mod` — which is also why the type is guaranteed to be there, the operation
having bound it. The generator simply does not do that today, in exchange for not
adding a filesystem-dependent code path that can fail when `--output` sits outside
a module.

**In practice:** entry parameters are scalars. The Fragment CLI generates
`String`, `Int64` and similar for every parameter in a Schema, so this affects
hand-written operations only.

### §2.3a — tags, groups and conditions use the SDK's input types

A payload's `Tags`, `Groups` and `Conditions` are `[]queries.LedgerEntryTagInput`
and friends, from the SDK's own `queries` package — not from the package the
payloads were generated alongside.

This one is forced rather than chosen: genqlient emits only the types its own
operations reach, so those input types are absent from the customer's generated
package unless their operations happen to reference them. The import path for the
customer's package is derivable from the enclosing `go.mod`, so the *parameter*
fallback below is a choice; this is not.

A customer generating with `--package fragment` therefore has both
`fragment.LedgerEntryTagInput` and payloads wanting
`queries.LedgerEntryTagInput`: structurally identical, mutually unassignable. The
generated package doc and the README both say so, because the compile error does
not explain itself.

### §3.2 — explicit null on a typed payload

**What the spec asks:** unset and explicit `null` must remain distinguishable.

**What this SDK does:** optional fields are pointers, following genqlient's
`Optional: "pointer"`. nil means unset and is omitted. A typed payload therefore has no
way to send a deliberate `null`.

**Why:** three-state optionals would mean an `Optional[T]` wrapper on every optional
field, diverging from how the rest of the SDK models optionality for a case the fixtures
do not exercise — `005-unset-omitted` tests only omission.

**Workaround:** none, and none is needed. `AddLedgerEntryInput` cannot express an
explicit null either — a nil `*string` there means both "unset" and "null" with no
way to tell them apart — so §3.2's distinction is not reachable from Go by any
route. See the §3.5 note below for what that forced.

### §3.5 — raw entries do not pass their nulls through

**What the spec asks:** an SDK must allow raw and typed entries in the same batch,
and notes that "raw inputs bypass §3.2 — a caller who explicitly passes `null` gets
`null`, and this asymmetry is accepted."

**What this SDK does:** `queries.RawEntry` omits unset fields, exactly as a typed
payload does. Values inside `parameters` are passed through untouched, since those
are the caller's own JSON.

**Why:** the asymmetry the spec accepts turns out to make §3.5's MUST unsatisfiable
in Go. Marshalling `AddLedgerEntryInput` directly encodes a `LedgerMatchInput` with
only `ik` set as `{"id":null,"ik":"..."}`. The API resolves that as a *different*
Ledger from the `{"ik":"..."}` a typed payload sends, and rejects the batch:

```
invalid_input_provided: Bad Request: entries[1] targets a different Ledger;
all entries in a batch must target the same Ledger.
```

Both entries named the same Ledger. Found by `TestAddTypedLedgerEntriesMixedWithRaw`
against a live API — it is not reproducible offline, since the JSON looks correct
until the API interprets it.

Nothing is given up by omitting the nulls. A nil `*string` in the generated input
types means both "unset" and "null" with no way to distinguish them, so there is no
explicit null a Go caller could have intended in the first place. The asymmetry
§3.5 describes is not expressible in this language, and emitting it anyway only
produced nulls nobody asked for.

**Worth raising upstream**, since it affects the spec rather than just this SDK:
§3.5's parenthetical assumes the asymmetry is harmless, and for a match input it is
not. Either the spec should say raw inputs must still omit unset fields, or it
should note that SDKs whose input types cannot omit are unable to satisfy §3.5.

### §3.4 — strict profile not adopted

Baseline only. Strict would need a replacement `graphql.Client` using
`SetEscapeHTML(false)`, because `encoding/json` escapes `<`, `>`, and `&` even from
inside a custom `MarshalJSON`. Under baseline the comparison is on parsed JSON, so that
escaping is not observable and fixture `006-non-ascii` passes without it. The spec
records strict as an open decision that Node's design may never satisfy, so there is
nothing to be byte-equal *with* yet.

If strict is adopted later, `batch.Object` already emits in a controlled order, so the
change is the client plus canonical key ordering rather than a rewrite.

## Answered by the live tests

The shared specification lists as a known gap that "no behavior here has been
verified against a live API. Server tolerance for an entry object with `lines` absent
and `type` present is untested in every SDK."

It is tested now, in `internal/livetest`, and the answer is that the API accepts it.
`TestAddTypedLedgerEntries` posts a two-entry batch built from generated payloads and
both entries commit with their lines materialised. Replay reporting and atomicity
hold as specified. One thing did not work as the spec assumed — see the §3.5 note
above.

## Ambiguities found while implementing

Both are worth resolving upstream; neither blocked this work.

1. **§5 cites the wrong path.** §5 and `CONTRIBUTING.md` place the fixtures at
   `spec/conformance/`, while `README.md` vendors them under `shared-spec/` with the
   SDK-owned runner as a sibling `spec/`. The sibling reading is self-consistent, so
   §5's path looks stale.

2. **A non-literal `typeVersion` has no defined identity.** §2.1 does not require
   `typeVersion` to be a literal, but §2.2 derives identity from "the integer literal …
   if present, otherwise 1". An operation with a literal `type` and a variable
   `typeVersion` falls between the two. Treating it as absent would pin the payload to
   version 1 even though the operation accepts any version, silently posting the wrong
   version, so this SDK skips such operations with a warning
   (`TestNonLiteralTypeVersionIsSkipped`).

## Regenerating

```shell
go test ./internal/typedentries -update   # refresh the committed generated payloads
go test ./...
```

## A note on testing a generator

Every codegen defect found in this work produced source that `gofmt` accepted and
the Go compiler rejected, and each was initially masked by a test that supplied
something the CLI does not — a scalar table containing `String`, or an expected
`Param` value that encoded the bug.

Two habits follow, and they are why `compile_test.go` and `main_test.go` exist:

- **Build the generated output, do not just inspect it.** A test that asserts on
  derived values or on emitted text cannot see a missing import, a field colliding
  with a method, or a helper called with the wrong type.
- **Feed tests the real inputs.** `main_test.go` reads the binding table from
  `newCodegenConfig` rather than restating it, and the built-in scalars are
  deliberately absent from every test table so the derivation has to resolve them
  itself.
