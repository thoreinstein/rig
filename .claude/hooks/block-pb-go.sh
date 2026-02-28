#!/bin/bash
# Block edits to generated .pb.go files.
# Edit the .proto source and run 'make generate' instead.

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$FILE_PATH" == *.pb.go ]]; then
  jq -n '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: "Do not edit generated .pb.go files. Edit the .proto source and run make generate."
    }
  }'
  exit 0
fi

exit 0
