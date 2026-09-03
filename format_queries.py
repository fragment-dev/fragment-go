"""
Script to format/clean the standard queries in the SDK.

1. Capitalize query names so that they are consistent with Go's public
   method nomenclature.
2. Rename protected variable names.
"""
import string
from typing import List

def format_line(line: str):
  is_first_line = line.startswith("mutation") or line.startswith("query")
  contains_type = line.find("$type: String!") != -1 or line.find("$type: SafeString!") != -1 or line.find("type: $type") != -1
  if not (is_first_line or contains_type):
    return line
  if is_first_line:
    line_parts = line.split(" ")
    return " ".join([part[0].upper()+part[1:] if idx == 1 else part for idx,part in enumerate(line_parts)])
  if contains_type:
    return line.replace("$type", "$entryType")

def format_graphql_queries(raw_lines: [str]):
  return map(format_line, raw_lines)

def main():
  formatted_lines = list()
  with open('queries/queries.graphql', 'r') as infile:
    formatted_lines = format_graphql_queries(infile.readlines())
  with open('queries/queries.graphql', 'w+') as outfile:
    outfile.write("".join(formatted_lines))

if __name__ == "__main__":
  main()
  
