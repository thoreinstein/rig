# Rig API Governance (v1)

This document outlines the policies and standards for the Rig gRPC API and plugin system.

## 1. Versioning Schema

Rig follows a strict versioning schema for its API to ensure stability and predictability for plugin developers.

### Major.Minor.Patch

- **Major**: Incremented for breaking changes (e.g., `v1`, `v2`). Breaking changes are only allowed in new major versions.
- **Minor**: Incremented for backward-compatible features (e.g., new services, new optional fields).
- **Patch**: Incremented for bug fixes and internal improvements that do not change the API surface.

In Protobuf, versions are represented by the package name (e.g., `rig.v1`).

## 2. Deprecation Policy

To balance innovation with stability, Rig employs a **3-version deprecation policy**.

1.  **Version N**: A feature/field is marked as deprecated using the `deprecated` option in Protobuf and documented in the code.
2.  **Version N+1**: The feature/field remains available but triggers a warning in the logs during runtime.
3.  **Version N+2**: The feature/field is removed from the API.

## 3. Backward Compatibility Rules

The following changes are considered **backward compatible**:
- Adding a new service.
- Adding a new method to an existing service.
- Adding a new field to a message (provided it's optional).
- Adding a new enum value.

The following changes are considered **breaking**:
- Renaming or removing a service, method, or field.
- Changing the type of a field.
- Changing the tag number of a field.
- Adding a required field (though Proto3 defaults to all fields being optional).

## 4. API Evolution

When evolving the API, prefer adding new fields or services over modifying existing ones. If a fundamental change is required, a new major version of the package should be created (e.g., moving from `pkg/api/v1` to `pkg/api/v2`).
