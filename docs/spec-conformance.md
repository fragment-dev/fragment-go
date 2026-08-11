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
| `main.go` (`addTypedPayloads`) | wires derivation into the codegen CLI |
| `batch/batch.go` | `Entry` interface and the ordered, omitting JSON writer |
| `queries/batch.go` | `AddTypedLedgerEntries`, `RawEntry` |
| `internal/conformance/` | the shared fixtures, and the committed generated output |

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
| 2.3 | Untyped fallback | `Payload.Untyped` emits a `json.RawMessage` field. `TestParametersWithoutInlineObjectFallBackToUntyped`. |
| 2.3a | Common fields | `commonFieldDecls` in `emit.go`, a fixed list. Not derived from the operation. `lines` deliberately absent. |
| 2.4 | Source order | `paramsOf` appends in `Children` order; `batch.Object` preserves insertion order, which a Go map could not. `TestParameterOrderIsSourceOrder` in both packages. |
| 2.5 | Name carries version | `payloadName` always appends `V<n>`. `TestPayloadNamesCarryTheirVersion`. |
| 2.5 | Unpinned normalises to 1 | `typeVersionOf`. Applied to identity, name, and wire value alike. |
| 2.5 | Local escaping | `escapeFields`; first occurrence keeps the plain name, later ones are suffixed. Warns on rename. |
| 2.5 | Colliding names stay distinct | `TestCollidingFieldNamesStayDistinct`, plus `TestParametersCannotShadowCommonFields` for the `posted` case. |
| 2.6 | Additive changes don't break callers | `TestNewOptionalParameterDoesNotRenameExistingFields`, `TestPayloadNamesCarryTheirVersion`. |
| 2.6 | Go must require keyed literals | Leading `_ struct{}` on every payload. `TestUnkeyedLiteralDoesNotCompile`. |
| 2.6 | Snapshot test | `TestGeneratedPayloadsAreCurrent` diffs regenerated output against the committed files in `internal/conformance/f00N/`. |
| 3.1 | Shape and entry order | Generated `MarshalJSON`; `TestEntryOrderPreserved`. |
| 3.2 | Unset omitted, never null | `batch.SetOpt` / `batch.SetSlice`. `omitempty` is not used anywhere on this path. `TestUnsetIsOmittedNotNull`, `TestSetOptOmitsNilAndKeepsZeroValues`. |
| 3.3 | Verbatim wire names | `Param.WireName` is never transformed; escaping only touches `FieldName`. Fixture `003`. |
| 3.4 | Baseline equivalence | Fixture comparison is parsed-JSON equality in `assertMatchesFixture`. |
| 3.5 | Mixing raw and typed | `queries.RawEntry`. `TestMixingRawAndTypedEntries`. |
| 3.6 | Everything accepted serialises | `AddTypedLedgerEntries` accepts only `batch.Entry`, whose sole method is the marshaller, so there is nothing acceptable that cannot serialise. |
| 4 | Batch semantics | Inherited from the API. `AddTypedLedgerEntries` returns the union undisturbed and documents narrowing; `AddLedgerEntriesError.Errors` is surfaced per entry. |

## Deviations

### §5 — no runner over the shared fixtures

**What the spec asks:** each SDK implements a runner that generates from
`input.graphql`, builds the batch in `case.json`, serializes, and compares to
`expected.json`.

**What this SDK does:** the fixtures are copied into `internal/conformance/testdata/`
and each case is written out by hand in `wire_test.go`, against generated output that
is committed under `internal/conformance/f00N/`.

**Why:** Go cannot do it in one process. The payloads are generated source and must be
compiled before anything can construct them, so a runner would need a build step
between reading `input.graphql` and building the batch. Resolving `("user-funds-account", 2)`
to a compiled Go type at runtime would additionally need reflection over field names,
which is exactly the mapping the generator already knows statically.

**Cost, stated plainly:** this SDK will *not* go red on its own when upstream changes a
fixture. The `.graphql`, `case.json`, and `expected.json` files under
`internal/conformance/testdata/` are copies, and re-syncing them is currently manual.
The `updateSDKQueries` workflow does not yet fetch `shared-spec/`.

### §3.2 — explicit null on a typed payload

**What the spec asks:** unset and explicit `null` must remain distinguishable.

**What this SDK does:** optional fields are pointers, following genqlient's
`Optional: "pointer"`. nil means unset and is omitted. A typed payload therefore has no
way to send a deliberate `null`.

**Why:** three-state optionals would mean an `Optional[T]` wrapper on every optional
field, diverging from how the rest of the SDK models optionality for a case the fixtures
do not exercise — `005-unset-omitted` tests only omission.

**Workaround:** `queries.RawEntry`, which §3.5 already blesses for mixing untyped inputs
into a batch. Raw entries serialize through the generated `AddLedgerEntryInput`, so their
unset fields are sent as `null`.

### §3.4 — strict profile not adopted

Baseline only. Strict would need a replacement `graphql.Client` using
`SetEscapeHTML(false)`, because `encoding/json` escapes `<`, `>`, and `&` even from
inside a custom `MarshalJSON`. Under baseline the comparison is on parsed JSON, so that
escaping is not observable and fixture `006-non-ascii` passes without it. The spec
records strict as an open decision that Node's design may never satisfy, so there is
nothing to be byte-equal *with* yet.

If strict is adopted later, `batch.Object` already emits in a controlled order, so the
change is the client plus canonical key ordering rather than a rewrite.

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
go test ./internal/conformance -update   # refresh the committed generated payloads
go test ./...
```
