# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-11-08

### Added
- Initial release of Planear
- Two core entry points: `plan.Generate()` and `apply.Run()`
- Type-safe generics for handling any record type
- CSV-driven declarative state management
- Custom callbacks for complete control: `OnAdd`, `OnUpdate`, `OnDelete`, `OnFinalize`
- Automatic exponential backoff retry logic (3 retries, 100ms base delay)
- Configurable parallel execution with worker pool
- Field-level change tracking for updates
- Comprehensive execution reporting
- Record validation support
- Streaming CSV parser for large files

### Documentation
- Complete README with quick start
- Real-world examples and use cases
- Comparison with Terraform, Pulumi, Liquibase
- Contributing guidelines
- Publishing guides

### Test Coverage
- 98.4% code coverage
- Comprehensive test suite covering:
  - Plan generation from CSV
  - Retry logic with exponential backoff
  - Parallel execution and task management
  - Error handling and recovery
  - Formatting and reporting

### Examples
- Basic example with successful operations
- Mixed results example demonstrating retry behavior with partial failures

[1.0.0]: https://github.com/algebananazzzzz/planear/releases/tag/v1.0.0
