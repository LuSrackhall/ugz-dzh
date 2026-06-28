## ADDED Requirements

### Requirement: CI workflow triggers
CI workflow SHALL trigger on:
- Pull requests targeting `main` branch
- Push events to `main` branch

#### Scenario: PR triggers CI
- **WHEN** a pull request is opened or updated targeting `main`
- **THEN** ci.yml workflow SHALL run

#### Scenario: Push triggers CI
- **WHEN** code is pushed to `main` branch
- **THEN** ci.yml workflow SHALL run

### Requirement: Test data creation
CI workflow SHALL create simplified test data before running tests.

#### Scenario: Create test vouchers
- **WHEN** CI workflow starts
- **THEN** it SHALL create `vouchers/2026_01/01.md` with simplified voucher content

#### Scenario: Create test config
- **WHEN** CI workflow starts
- **THEN** it SHALL create `科目余额总览.json` with minimal valid JSON structure

### Requirement: Run tests
CI workflow SHALL run all Go tests.

#### Scenario: Execute go test
- **WHEN** test data is created
- **THEN** workflow SHALL execute `go test ./...`

#### Scenario: Test failure stops workflow
- **WHEN** any test fails
- **THEN** workflow SHALL fail and report the error

### Requirement: Go version setup
CI workflow SHALL set up Go 1.26.

#### Scenario: Setup Go environment
- **WHEN** workflow starts
- **THEN** it SHALL use `actions/setup-go@v5` with `go-version: '1.26'`

### Requirement: Go module caching
CI workflow SHALL cache Go modules for faster builds.

#### Scenario: Enable Go cache
- **WHEN** setting up Go
- **THEN** it SHALL set `cache: true` in `actions/setup-go`
