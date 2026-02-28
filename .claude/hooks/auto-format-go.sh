#!/bin/bash
# Auto-format Go files after edit using gofmt.
# Skips generated .pb.go files.

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *.go ]] && [[ "$FILE_PATH" != *.pb.go ]]; then
  gofmt -w "$FILE_PATH" 2>/dev/null
fi

exit 0
