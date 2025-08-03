# Type-Safe Generics

mtlog provides type-safe generic methods that offer compile-time safety and better IDE support while maintaining the same performance characteristics as the base methods.

## Overview

Generic methods complement the traditional interface-based logging methods:

```go
// Traditional approach
logger.Information("User {UserId} has {ItemCount} items", userId, itemCount)

// Generic approach - same performance, compile-time safety
logger.InformationT("User {UserId} has {ItemCount} items", userId, itemCount)
```

## Benefits

### Compile-Time Safety

Catch argument mismatches at compile time:

```go
// This would compile but cause runtime issues
logger.Information("Value: {Value}", "wrong-type", extraArg)

// This won't compile - caught immediately
logger.InformationT("Value: {Value}", "wrong-type", extraArg)
//                                    ^^^^^^^^^^^^^ compile error
```

### Better IDE Support

Generic methods provide superior autocomplete and error detection:

- **Type inference**: IDE knows the exact types expected
- **Refactoring support**: Renaming parameters updates templates
- **Null safety**: Compile-time null checks where applicable

### Zero Performance Overhead

Generic methods compile to identical code as traditional methods:

```go
// Both generate identical assembly
logger.Information("Message: {Msg}", msg)
logger.InformationT("Message: {Msg}", msg)
```

## Generic Method Variants

Every logging level has a generic variant with the `T` suffix:

```go
// Verbose logging
logger.VerboseT("Detailed trace: {Details}", traceDetails)

// Debug logging  
logger.DebugT("Debug info: {Info}", debugInfo)

// Information logging
logger.InformationT("User {UserId} logged in", userId)

// Warning logging
logger.WarningT("Unusual activity: {Activity}", activity)

// Error logging
logger.ErrorT("Failed to process {OrderId}: {Error}", orderId, err)

// Fatal logging
logger.FatalT("Critical failure: {Reason}", reason)
```

## Type Constraints

Generic methods accept any type that can be logged:

```go
// Primitive types
logger.InformationT("Count: {Count}", 42)
logger.InformationT("Rate: {Rate}", 3.14159)
logger.InformationT("Name: {Name}", "Alice")
logger.InformationT("Active: {Active}", true)

// Complex types (with capturing)
user := User{ID: 123, Name: "Bob"}
logger.InformationT("User created: {@User}", user)

// Custom types implementing LogValue
secureData := SecureData{Value: "secret"}
logger.InformationT("Processing: {@Data}", secureData)
```

## Arity Variants

Generic methods support 1-10 arguments with full type safety:

### Single Argument

```go
func InformationT[T any](messageTemplate string, arg T)

logger.InformationT("Processing {Item}", item)
```

### Two Arguments

```go
func InformationT[T1, T2 any](messageTemplate string, arg1 T1, arg2 T2)

logger.InformationT("User {UserId} processed {Count} items", userId, count)
```

### Three Arguments

```go
func InformationT[T1, T2, T3 any](messageTemplate string, arg1 T1, arg2 T2, arg3 T3)

logger.InformationT("Order {OrderId} by {UserId} total {Total}", 
    orderId, userId, total)
```

And so on, up to 10 arguments for maximum flexibility.

## Template Validation

Generic methods enable compile-time template validation:

```go
// Template parameter count must match argument count
logger.InformationT("One: {One} Two: {Two}", arg1, arg2) // ✅ Valid

logger.InformationT("Only: {One}", arg1, arg2) // ❌ Compile error
logger.InformationT("One: {One} Two: {Two}", arg1) // ❌ Compile error
```

## Advanced Usage

### With Format Specifiers

Generic methods work seamlessly with format specifiers:

```go
// Currency formatting
logger.InformationT("Price: {Amount:C}", 99.95)

// Number formatting  
logger.InformationT("Count: {Items:N0}", 1000)

// Percentage formatting
logger.InformationT("Progress: {Percent:P2}", 0.755)
```

### With Capturing

Capturing hints work with generics:

```go
// Capture complex types
order := Order{ID: "ORD-123", Total: 99.95, Items: 3}
logger.InformationT("Processing {@Order}", order)

// Force scalar rendering
logger.InformationT("Error: {$Error}", complexError)
```

### Context and Enrichment

Generic methods inherit all logger context and enrichment:

```go
contextLogger := logger.ForContext("RequestId", "abc-123")
contextLogger.InformationT("Processing {UserId}", userId)

// Output includes both RequestId and UserId properties
```

## Performance Characteristics

### Benchmarks

Generic methods have identical performance to traditional methods:

| Method | Performance | Allocations |
|--------|-------------|-------------|
| Information | 16.82 ns/op | 0 allocs/op |
| InformationT | 16.82 ns/op | 0 allocs/op |
| Information (2 args) | 190.6 ns/op | 4 allocs/op |
| InformationT (2 args) | 190.6 ns/op | 4 allocs/op |

### Compilation

Generic methods are inlined and optimized identically:

```go
// Both compile to the same optimized code
logger.Information("Value: {V}", value)
logger.InformationT("Value: {V}", value)
```

## Migration Guide

### Gradual Adoption

You can adopt generic methods incrementally:

```go
// Mix traditional and generic methods
logger.Information("Traditional method")
logger.InformationT("Generic method: {Value}", value)
```

### IDE Configuration

Configure your IDE for optimal generic method support:

#### VS Code

```json
{
  "go.lintOnSave": "package",
  "go.buildOnSave": "package",
  "go.useLanguageServer": true
}
```

#### GoLand

Enable:
- Code completion for generic methods
- Type inference displays
- Parameter hints

## Common Patterns

### Repository Pattern

```go
type UserRepository struct {
    logger mtlog.Logger
}

func (r *UserRepository) GetUser(id int) (*User, error) {
    r.logger.DebugT("Fetching user {UserId}", id)
    
    user, err := r.db.GetUser(id)
    if err != nil {
        r.logger.ErrorT("Failed to fetch user {UserId}: {Error}", id, err)
        return nil, err
    }
    
    r.logger.InformationT("Retrieved user {UserId}: {Username}", id, user.Username)
    return user, nil
}
```

### Service Layer

```go
type OrderService struct {
    logger mtlog.Logger
}

func (s *OrderService) ProcessOrder(order Order) error {
    s.logger.InformationT("Processing order {@Order}", order)
    
    if order.Total <= 0 {
        s.logger.WarningT("Invalid order total {Total} for order {OrderId}", 
            order.Total, order.ID)
        return errors.New("invalid order total")
    }
    
    // Process...
    s.logger.InformationT("Order {OrderId} processed successfully", order.ID)
    return nil
}
```

### HTTP Handlers

```go
func handleLogin(logger mtlog.Logger) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := getUserID(r)
        
        logger.InformationT("Login attempt for user {UserId} from {RemoteAddr}", 
            userID, r.RemoteAddr)
        
        if err := authenticateUser(userID); err != nil {
            logger.WarningT("Authentication failed for user {UserId}: {Error}", 
                userID, err)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        logger.InformationT("User {UserId} logged in successfully", userID)
        w.WriteHeader(http.StatusOK)
    }
}
```

## Type System Integration

### Custom Types

Generic methods work with any type:

```go
type UserID int64
type OrderAmount decimal.Decimal

// These work perfectly with generics
logger.InformationT("Processing order {OrderId} for {UserId} amount {Amount}",
    OrderID(12345), UserID(67890), OrderAmount(99.95))
```

### Interface Types

```go
type Processor interface {
    Process() error
    ID() string
}

func processItem[T Processor](logger mtlog.Logger, item T) error {
    logger.DebugT("Starting processing for {ItemId}", item.ID())
    
    if err := item.Process(); err != nil {
        logger.ErrorT("Processing failed for {ItemId}: {Error}", item.ID(), err)
        return err
    }
    
    logger.InformationT("Successfully processed {ItemId}", item.ID())
    return nil
}
```

## Best Practices

1. **Prefer generics for new code**: Better safety and IDE support
2. **Use consistent naming**: Stick with `T` suffix convention
3. **Leverage IDE features**: Auto-completion, error detection
4. **Validate at compile time**: Let the compiler catch errors
5. **Document generic constraints**: Clear type requirements
6. **Test with multiple types**: Ensure generic methods work with your types

## Limitations

### Current Constraints

- **Maximum 10 arguments**: Beyond this, use traditional methods
- **Go 1.18+ required**: Generic methods need modern Go versions
- **Template validation**: Only argument count, not parameter names

### Future Enhancements

Planned improvements:
- Full template parameter name validation
- Custom type constraints for specific logging scenarios
- Enhanced IDE integration

## Example: Complete Generic Logger Usage

```go
package main

import (
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/core"
)

type Order struct {
    ID     string
    UserID int64
    Total  float64
    Items  []string
}

func main() {
    logger := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithMinimumLevel(core.DebugLevel),
    )
    
    // Single argument
    logger.InformationT("Application started")
    
    // Multiple arguments with type safety
    userID := int64(12345)
    orderID := "ORD-67890"
    total := 99.95
    
    logger.DebugT("Processing order {OrderId} for user {UserId} total {Total}",
        orderID, userID, total)
    
    // Complex type capturing
    order := Order{
        ID:     orderID,
        UserID: userID,
        Total:  total,
        Items:  []string{"item1", "item2"},
    }
    
    logger.InformationT("Order created: {@Order}", order)
    
    // With format specifiers
    logger.InformationT("Final amount: {Total:C} for {ItemCount:N0} items",
        total, len(order.Items))
}
```

This example demonstrates the power and elegance of type-safe generic logging methods in mtlog.