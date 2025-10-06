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

## Opening a Pull Request

When you're ready to contribute:

1. **Open a Pull Request** with a clear summary of what you changed and why
2. **Reach out to us on Slack** if possible to let us know about your contribution
3. We'll review your changes and work with you to get them merged
