# Contributing to mtlog

Thank you for your interest in contributing to mtlog!

## Before You Start

- Check existing [issues](https://github.com/willibrandon/mtlog/issues) and [pull requests](https://github.com/willibrandon/mtlog/pulls)
- For significant changes, open an issue first to discuss your proposal

## Development Setup

```bash
# Clone the repository
git clone https://github.com/willibrandon/mtlog.git
cd mtlog

# Run tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run linter
golangci-lint run
```

## Code Guidelines

- Follow standard Go conventions and idioms
- Keep the zero-allocation promise for simple logging paths
- Add tests for new features (unit and integration where appropriate)
- Update documentation and examples as needed
- Ensure `golangci-lint` passes

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linter
5. Commit with a descriptive message
6. Push to your fork
7. Open a pull request with a clear description

## Testing

- Unit tests should be fast and focused
- Integration tests require Docker for Seq, Elasticsearch, and Splunk
- Use `MemorySink` for testing log output
- Benchmark critical paths to ensure performance

## Questions?

Open an issue or discussion - we're happy to help!