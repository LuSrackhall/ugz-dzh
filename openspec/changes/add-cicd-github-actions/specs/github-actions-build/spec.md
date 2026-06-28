## ADDED Requirements

### Requirement: Build workflow triggers
Build workflow SHALL trigger on push events to `main` branch.

#### Scenario: Push triggers build
- **WHEN** code is pushed to `main` branch
- **THEN** build.yml workflow SHALL run

### Requirement: Snapshot build verification
Build workflow SHALL verify multi-platform builds succeed without publishing.

#### Scenario: Run GoReleaser snapshot
- **WHEN** build workflow runs
- **THEN** it SHALL execute `goreleaser release --snapshot --clean`

#### Scenario: Build failure stops workflow
- **WHEN** GoReleaser build fails
- **THEN** workflow SHALL fail and report the error

### Requirement: GoReleaser action usage
Build workflow SHALL use `goreleaser/goreleaser-action@v6`.

#### Scenario: Use GoReleaser action
- **WHEN** running build
- **THEN** workflow SHALL use `goreleaser/goreleaser-action@v6` with `version: '~> v2'`

### Requirement: No publishing in build
Build workflow SHALL NOT create releases or upload artifacts.

#### Scenario: Snapshot mode only
- **WHEN** GoReleaser runs
- **THEN** it SHALL use `--snapshot` flag to prevent publishing
