## ADDED Requirements

### Requirement: GoReleaser version
GoReleaser configuration SHALL use version 2 format.

#### Scenario: Version declaration
- **WHEN** reading `.goreleaser.yml`
- **THEN** it SHALL start with `version: 2`

### Requirement: Build configuration
GoReleaser SHALL build for multiple platforms with CGO disabled.

#### Scenario: Disable CGO
- **WHEN** building binaries
- **THEN** it SHALL set `CGO_ENABLED=0` for static linking

#### Scenario: Target platforms
- **WHEN** building
- **THEN** it SHALL build for:
  - linux/amd64, linux/arm64
  - darwin/amd64, darwin/arm64
  - windows/amd64, windows/arm64

### Requirement: Binary naming
GoReleaser SHALL name output binary as `ledger`.

#### Scenario: Binary name
- **WHEN** build completes
- **THEN** output binary SHALL be named `ledger` (or `ledger.exe` on Windows)

### Requirement: Archive format
GoReleaser SHALL create platform-specific archives.

#### Scenario: tar.gz for Unix
- **WHEN** packaging for Linux or macOS
- **THEN** it SHALL create `.tar.gz` archive

#### Scenario: zip for Windows
- **WHEN** packaging for Windows
- **THEN** it SHALL create `.zip` archive

### Requirement: Archive naming
GoReleaser SHALL use descriptive archive names.

#### Scenario: Archive name template
- **WHEN** creating archives
- **THEN** names SHALL follow template: `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`

### Requirement: Checksum file
GoReleaser SHALL generate checksums file.

#### Scenario: Checksum generation
- **WHEN** release completes
- **THEN** it SHALL generate `checksums.txt`

### Requirement: Changelog filters
GoReleaser SHALL filter changelog entries.

#### Scenario: Exclude docs commits
- **WHEN** generating changelog
- **THEN** it SHALL exclude commits matching `^docs:`

#### Scenario: Exclude test commits
- **WHEN** generating changelog
- **THEN** it SHALL exclude commits matching `^test:`
