#!/bin/bash

echo "Running mtlog benchmarks..."
echo

echo "=== Quick Benchmarks (5s each) ==="
go test -bench="BenchmarkSimpleLog|BenchmarkLogWithProperties|BenchmarkLogBelowMinimumLevel" -benchmem -run=^$ -benchtime=5s

echo
echo "=== Full Benchmark Suite ==="
go test -bench=. -benchmem -run=^$ -benchtime=10s

echo
echo "=== Allocation Analysis ==="
go test -run=TestAllocationBreakdown -v

echo
echo "Benchmark complete!"