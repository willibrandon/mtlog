# Benchmark Results: mtlog vs zap vs zerolog

## Test Environment
- CPU: AMD Ryzen 9 9950X 16-Core Processor
- OS: Windows
- Go: (latest)
- Date: 2025-07-21

## Summary

mtlog achieves its goal of zero-allocation logging for simple messages while maintaining competitive performance with zap and zerolog. The library excels at:

1. **Simple string logging** - Fastest with zero allocations
2. **Filtered messages** - Extremely fast early rejection
3. **Message templates** - Preserves semantic meaning while competitive performance

## Detailed Results

### 1. Simple String Logging (No Properties)
| Logger | Time/op | Allocations | Bytes/op | vs mtlog |
|--------|---------|-------------|----------|----------|
| **mtlog** | 16.82 ns | 0 | 0 B | baseline |
| zap | 146.6 ns | 0 | 0 B | 8.7x slower |
| zap-sugar | 151.7 ns | 0 | 0 B | 9.0x slower |
| zerolog | 36.46 ns | 0 | 0 B | 2.2x slower |

✅ **mtlog wins** - Achieved zero-allocation goal with best performance

### 2. Two Properties
| Logger | Time/op | Allocations | Bytes/op |
|--------|---------|-------------|----------|
| mtlog | 190.6 ns | 4 | 448 B |
| zap | 216.9 ns | 1 | 128 B |
| zap-sugar | 285.0 ns | 1 | 257 B |
| **zerolog** | 49.48 ns | 0 | 0 B |

⚡ zerolog is highly optimized for structured logging with zero allocations

### 3. Ten Properties
| Logger | Time/op | Allocations | Bytes/op |
|--------|---------|-------------|----------|
| mtlog | 762.2 ns | 10 | 1650 B |
| zap | 469.6 ns | 1 | 707 B |
| **zerolog** | 145.0 ns | 0 | 0 B |

### 4. With Context (Pre-enriched Logger)
| Logger | Time/op | Allocations | Bytes/op |
|--------|---------|-------------|----------|
| mtlog | 205.2 ns | 3 | 416 B |
| zap | 130.8 ns | 0 | 0 B |
| **zerolog** | 35.25 ns | 0 | 0 B |

### 5. Complex Object (Struct Destructuring)
| Logger | Time/op | Allocations | Bytes/op |
|--------|---------|-------------|----------|
| mtlog | 422.8 ns | 11 | 913 B |
| zap | 391.2 ns | 3 | 225 B |
| **zerolog** | 194.7 ns | 3 | 192 B |

### 6. Filtered Out Messages (Below Minimum Level)
| Logger | Time/op | Allocations | Bytes/op | vs mtlog |
|--------|---------|-------------|----------|----------|
| **mtlog** | 1.470 ns | 0 | 0 B | baseline |
| zap | 3.572 ns | 0 | 0 B | 2.4x slower |
| zerolog | 1.711 ns | 0 | 0 B | 1.2x slower |

✅ **mtlog wins** - Extremely fast early rejection of filtered messages

### 7. Console Output (Formatted)
| Logger | Time/op | Allocations | Bytes/op |
|--------|---------|-------------|----------|
| mtlog | 758.2 ns | 20 | 866 B |
| **zap** | 351.9 ns | 4 | 129 B |
| zerolog | 2515 ns | 44 | 2652 B |

## Analysis

### mtlog Strengths
1. **Zero-allocation simple logging** - Best-in-class performance
2. **Fast filtering** - Minimal overhead for discarded messages
3. **Message templates** - Preserves structure unlike format strings
4. **Pipeline architecture** - Clean separation of concerns

### Areas for Optimization
1. **Property allocation** - Currently allocates for each property
2. **Complex objects** - Reflection-based destructuring has overhead
3. **Template parsing** - Could benefit from more aggressive caching

### Design Trade-offs
- **Message templates vs structured fields**: mtlog prioritizes readability and semantic grouping
- **Pipeline flexibility vs performance**: The pipeline adds some overhead but enables powerful features
- **Type safety vs allocations**: Dynamic property handling requires allocations

## Recommendations

1. **Use mtlog when**:
   - You want human-readable log messages with preserved structure
   - Zero-allocation simple logging is important
   - You need powerful filtering and enrichment capabilities
   - Message templates provide better semantic grouping

2. **Consider alternatives when**:
   - You need absolute minimum allocations for all scenarios (zerolog)
   - You're already invested in the zap ecosystem
   - Performance with many properties is critical

## Future Optimizations

1. **Property pooling** - Reuse property maps
2. **Type descriptor caching** - Cache reflection results
3. **Specialized formatters** - Avoid interface{} conversions
4. **SIMD template parsing** - Vectorized parsing for long templates