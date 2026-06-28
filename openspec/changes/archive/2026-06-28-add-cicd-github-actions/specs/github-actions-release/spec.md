## ADDED Requirements

### Requirement: Release workflow triggers
Release workflow SHALL trigger on tag push events matching `v*` pattern.

#### Scenario: Tag triggers release
- **WHEN** a tag matching `v*` is pushed
- **THEN** release.yml workflow SHALL run

### Requirement: Release workflow permissions
Release workflow SHALL have write permissions for repository contents.

#### Scenario: Minimal permissions
- **WHEN** release workflow runs
- **THEN** it SHALL declare `permissions: contents: write`

### Requirement: Full checkout
Release workflow SHALL fetch full git history for changelog generation.

#### Scenario: Fetch all history
- **WHEN** release workflow starts
- **THEN** it SHALL use `actions/checkout@v4` with `fetch-depth: 0`

### Requirement: GoReleaser release execution
Release workflow SHALL create GitHub Release with multi-platform binaries.

#### Scenario: Run GoReleaser release
- **WHEN** release workflow runs
- **THEN** it SHALL execute `goreleaser release --clean`

#### Scenario: Use GITHUB_TOKEN
- **WHEN** GoReleaser runs
- **THEN** it SHALL use `GITHUB_TOKEN` from `${{ secrets.GITHUB_TOKEN }}`

### Requirement: Release artifact formats
Release workflow SHALL produce platform-specific archives.

#### Scenario: Linux/macOS format
- **WHEN** building for Linux or macOS
- **THEN** output SHALL be `.tar.gz` archive

#### Scenario: Windows format
- **WHEN** building for Windows
- **THEN** output SHALL be `.zip` archive

### Requirement: Checksum generation
Release workflow SHALL generate checksums file.

#### Scenario: Create checksums
- **WHEN** release completes
- **THEN** it SHALL generate `checksums.txt` with SHA256 hashes

### Requirement: Changelog generation
Release workflow SHALL auto-generate changelog from commits.

#### Scenario: Generate changelog
- **WHEN** release runs
- **THEN** it SHALL generate changelog excluding `docs:` and `test:` commits
