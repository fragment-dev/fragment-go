# Contributing to the Fragment Go SDK

We welcome contributions to the SDK! Whether you're fixing a bug, improving documentation, or adding new features, we appreciate your input.

## Getting Started

### Code Generation

This SDK uses the [genqlient](https://github.com/Khan/genqlient) library to generate all available query functions, which live in `queries/queries.go`.

If you make a change that requires you to regenerate those functions, you can run:

```shell
go run main.go \
  --input=queries/queries.graphql \
  --output=queries/queries.go \
  --package=queries
```

### Typed batch Ledger Entry payloads

Alongside the genqlient pass, the codegen derives a typed payload per Ledger Entry type
and writes them to a `typed_payloads` package beside the generated client. The rules for
that derivation are shared with the other Fragment SDKs and specified in
[`fragment-dev/graphql-queries`](https://github.com/fragment-dev/graphql-queries).

The generated output for the specification's fixtures is committed under
`internal/conformance/`, and acts as both the snapshot test and the input to the wire
tests. If you change the generator, review the diff and refresh it:

```shell
go test ./internal/conformance -update
go test ./...
```

See [`docs/spec-conformance.md`](docs/spec-conformance.md) for the section-by-section
mapping and the deviations this SDK carries.

### Live API tests

`internal/livetest/` posts real Ledger Entries. Everything else in the repo stops at
the request boundary, so these are what confirm the API accepts the entry shape a
typed payload produces — an entry object with a `type` and no `lines`.

They skip unless credentials are set, so `go test ./...` stays offline:

```shell
FRAGMENT_CLIENT_ID=... \
FRAGMENT_CLIENT_SECRET=... \
FRAGMENT_SCOPE=... \
FRAGMENT_AUTH_URL=... \
FRAGMENT_API_URL=... \
  go test ./internal/livetest/ -v
```

Each run stores a new version of the `fragment-go-livetest` Schema and creates a
fresh Ledger, so runs cannot collide — but Ledgers accumulate. **Point these at a
scratch Workspace, not production.**

In CI they run as the `Live API tests` job in `.github/workflows/ci.yml`, once per
pull request rather than once per Go version. The job reads the same five values
from repository secrets:

```shell
gh secret set FRAGMENT_CLIENT_ID     --repo fragment-dev/fragment-go
gh secret set FRAGMENT_CLIENT_SECRET --repo fragment-dev/fragment-go
gh secret set FRAGMENT_SCOPE         --repo fragment-dev/fragment-go
gh secret set FRAGMENT_AUTH_URL      --repo fragment-dev/fragment-go
gh secret set FRAGMENT_API_URL       --repo fragment-dev/fragment-go
```

Pull requests from forks get no secrets, so the tests skip there and the job still
passes. On a branch in this repository a skip is treated as a failure, so a deleted
or renamed secret surfaces instead of quietly turning the job into a no-op.

## Opening a Pull Request

When you're ready to contribute:

1. **Open a Pull Request** with a clear summary of what you changed and why
2. **Reach out to us on Slack** if possible to let us know about your contribution
3. We'll review your changes and work with you to get them merged
