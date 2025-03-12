"""
This script is used to capitalize the query names in queries.graphql file.
"""
import string
from typing import List

def format_line(line: str):
  is_first_line = line.startswith("mutation") or line.startswith("query")
  if not is_first_line:
    return line
  line_parts = line.split(" ")
  return " ".join([part[0].upper()+part[1:] if idx == 1 else part for idx,part in enumerate(line_parts)])

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
  
