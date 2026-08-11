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

## Opening a Pull Request

When you're ready to contribute:

1. **Open a Pull Request** with a clear summary of what you changed and why
2. **Reach out to us on Slack** if possible to let us know about your contribution
3. We'll review your changes and work with you to get them merged
