## ADDED Requirements

### Requirement: Parse voucher entries from Markdown file

The system SHALL parse a Markdown file containing HTML `<table>` elements and extract all voucher entries into structured records.

#### Scenario: Parse a file with multiple voucher tables
- **WHEN** `ParseFile` is called with a path to a `.md` file containing one or more HTML `<table>` blocks with voucher rows
- **THEN** all valid entry rows are returned as `[]Entry`, with amounts parsed into integer cents

#### Scenario: Parse a file with no valid tables
- **WHEN** `ParseFile` is called with a path to a `.md` file that contains no `<table>` elements
- **THEN** an empty slice is returned with no error

#### Scenario: Parse a file with mixed valid and invalid rows
- **WHEN** a table contains header rows ("摘要", "总帐科目", etc.), empty rows, and data rows
- **THEN** header rows and rows with empty general accounts are skipped; only data rows with non-zero debit or credit amounts are included

### Requirement: Extract voucher number from document text

The system SHALL extract the voucher number from the document body using multiple regex patterns, falling back to the filename if body extraction fails.

#### Scenario: Voucher number in standard format
- **WHEN** the document text contains patterns like "记字第0001号"
- **THEN** the numeric part (e.g., 1) is extracted as the voucher number

#### Scenario: Voucher number not in body text
- **WHEN** the document body does not match any known voucher number pattern
- **THEN** the filename is scanned for a numeric candidate (excluding year-like numbers), and that candidate is used as the voucher number

### Requirement: Parse amounts to integer cents

The system SHALL parse monetary amount strings into `int64` cents, handling Chinese punctuation and thousand separators.

#### Scenario: Clean decimal amount
- **WHEN** the amount cell contains "1,234.56"
- **THEN** the result is `123456` (cents)

#### Scenario: Amount with Chinese comma
- **WHEN** the amount cell contains "1，234.56" (full-width comma)
- **THEN** the result is `123456` (cents)

#### Scenario: Empty amount cell
- **WHEN** the amount cell is empty or whitespace-only
- **THEN** the result is `0` with `false` indicating no valid parse

### Requirement: Clean detail account names

The system SHALL normalize detail account names by trimming whitespace and collapsing consecutive whitespace to a single space, without altering any characters.

#### Scenario: Detail account with extra whitespace
- **WHEN** the detail account cell contains "  张三  " (with leading/trailing spaces)
- **THEN** the result is "张三"

#### Scenario: Detail account with full-width spaces
- **WHEN** the detail account cell contains "李　四" (full-width space)
- **THEN** the full-width space is converted to a half-width space: "李 四"
