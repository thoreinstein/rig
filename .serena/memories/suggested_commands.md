# Suggested Commands for Rig

## Development
- **Build**: `go build -o rig`
- **Install (Dev)**: `go install .`
- **Lint**: `golangci-lint run`
- **Format**: `gofmt -w .`

## Testing
- **Run All Tests**: `go test ./...`
- **Run Tests Verbose**: `go test -v ./...`
- **Run Specific Package**: `go test ./pkg/git/...`
- **Run Specific Test**: `go test -run TestName ./pkg/...`

## Workflow & Task Management (Beads)
- **List Tasks**: `bd list --status open`
- **Start Task**: `bd update <id> --status in_progress`
- **Close Task**: `bd close <id>`
- **Sync Tasks**: `bd sync`

## Git
- **Status**: `git status`
- **Diff**: `git diff HEAD`
- **Log**: `git log --oneline -n 10`

## Dependencies
- **Update Modules**: `go get -u ./...`
- **Tidy Modules**: `go mod tidy`
- **Generate Protobufs**: `buf generate`
- **Lint Protobufs**: `buf lint`
