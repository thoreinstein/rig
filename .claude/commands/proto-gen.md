---
description: Regenerate Go code from proto files and validate
---

Run the full protobuf regeneration and validation pipeline:

1. Run `make generate` to regenerate Go code from .proto files
2. Run `buf lint` to lint the proto files
3. Run `go build ./...` to verify the generated code compiles
4. Run `go test ./pkg/api/...` to verify API tests still pass

If any step fails, stop and report the error. Do not proceed to the next step.

After all steps pass, run `git diff --stat` to show what changed.
