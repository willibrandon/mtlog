module github.com/willibrandon/mtlog/benchmarks

go 1.24.1

replace github.com/willibrandon/mtlog => ../

require (
	github.com/rs/zerolog v1.34.0
	github.com/willibrandon/mtlog v0.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
)
