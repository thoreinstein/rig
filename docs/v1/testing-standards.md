# Testing Standards and Patterns (v1)

This document defines the standards, patterns, and known pitfalls for testing in the Rig project.

## 1. Core Principles

- **Table-Driven Tests**: Mandatory for all logic. Use `t.Run` for subtests.
- **Interface-Based Mocking**: Dependencies should be mocked via interfaces.
- **Generated Mocks**: Prefer `mockery` (v3) for generating mocks.
- **External Testability**: Mocks must be importable by external test packages (e.g., `pkg/foo_test`).

## 2. Mockery Configuration (v3)

We use a package-centric `.mockery.yaml` at the project root.

### Standard Configuration Pattern

```yaml
all: false
template: testify
include-auto-generated: true # Required for gRPC and other generated interfaces
packages:
  thoreinstein.com/rig/pkg/yourpackage:
    config:
      dir: "{{.InterfaceDir}}/mocks"
      filename: "mock_{{snakecase .InterfaceName}}.go"
      pkgname: "mocks"
    interfaces:
      YourInterface: {}
```

### Key Decisions
- **`template: testify`**: Provides type-safe `EXPECT()` helpers.
- **`pkgname: "mocks"`**: Mocks live in their own package (`mocks`) within a subdirectory. This allows external packages to import them without test-file compilation issues.
- **`filename: "mock_{{snakecase .InterfaceName}}.go"`**: Consistent, predictable naming.

## 3. Patterns

### External Mock Packages
**Pattern**: Store generated mocks in a dedicated `mocks/` sub-package (e.g., `pkg/foo/mocks`).
**Rationale**: This allows external packages (e.g., `pkg/bar_test`) to import and use the mocks without requiring the original package's test files to be compiled or causing circular dependencies.

### Hand-Written Mocks for Complex Logic
**Pattern**: When `mockery` cannot handle an interface correctly (see Traps), hand-write the mock in the same `mocks/` directory.
**Rationale**: Maintains the same import path for consumers while providing the necessary logic that code generation lacks.

## 4. Traps and Pitfalls

### Trap: Mockery v3 Variadic gRPC Options
**Problem**: Mockery v3's `testify` template mishandles variadic arguments (like `...grpc.CallOption`) in gRPC client interfaces.
**Symptom**: The generated `Called(ctx, in, opts...)` passes `opts` as a single slice argument to the underlying `mock.Mock`, but the generated `EXPECT()` helper expects individual arguments for each option. This leads to "no match found" errors.
**Resolution**: Hand-write the mock for these specific interfaces. Manually expand the `[]grpc.CallOption` slice into individual arguments before calling `m.Called()`.

**Example Workaround:**
```go
func (m *MockClient) SomeMethod(ctx context.Context, in *Req, opts ...grpc.CallOption) (*Res, error) {
    // Manually expand args so they match EXPECT()
    args := make([]interface{}, 0, 2+len(opts))
    args = append(args, ctx, in)
    for _, opt := range opts {
        args = append(args, opt)
    }
    ret := m.Called(args...)
    return ret.Get(0).(*Res), ret.Error(1)
}
```

### Trap: Test-File Compilation Issues
**Problem**: Generating mocks into `_test.go` files or using the same package name as the implementation.
**Symptom**: Other packages cannot import the mocks because Go doesn't allow importing `_test.go` files, or it creates circular dependencies if the mock package imports the implementation package and vice versa.
**Resolution**: Always use a dedicated `mocks` package in a subdirectory.
