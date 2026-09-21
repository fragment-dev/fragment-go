"""
Formats the standard queries for the Go SDK.

1. Capitalizes operation names to match Go's public method nomenclature.
2. Renames the $type variable: `type` is a Go keyword and genqlient refuses
   to generate from it. Every operation using $type must have an explicit
   replacement in TYPE_RENAMES; an unmapped operation fails the run so the
   generated Go parameter is always deliberately named.
"""
import re

TYPE_RENAMES = {
    "AddLedgerEntry": "$entryType",
    "AddLedgerEntryRuntime": "$entryType",
    "ReconcileTx": "$entryType",
    "ReconcileTxRuntime": "$entryType",
    "CreatePayment": "$paymentType",
}


def format_lines(lines):
    curr_op_name = None
    for line in lines:
        if line.startswith(("mutation ", "query ")):
            parts = line.split(" ")
            parts[1] = parts[1][0].upper() + parts[1][1:]
            line = " ".join(parts)
            curr_op_name = re.match(r"\w+", parts[1]).group()
        if re.search(r"\$type\b", line):
            if curr_op_name not in TYPE_RENAMES:
                raise SystemExit(
                    f"operation {curr_op_name!r} uses $type but has no entry in "
                    "TYPE_RENAMES; add one so the generated Go parameter "
                    "gets a deliberate name ($type itself is a Go keyword)"
                )
            line = re.sub(r"\$type\b", TYPE_RENAMES[curr_op_name], line)
        yield line


def main():
    with open("queries/queries.graphql", "r") as infile:
        formatted = list(format_lines(infile.readlines()))
    with open("queries/queries.graphql", "w+") as outfile:
        outfile.write("".join(formatted))


if __name__ == "__main__":
    main()
