# Template Syntax Guide

mtlog supports two template syntaxes for maximum flexibility and developer comfort:

## Traditional Syntax

The traditional syntax uses single braces `{Property}`:

```go
logger.Information("User {UserId} logged in at {Time}", userId, loginTime)
logger.Information("Order {OrderId:000} total: ${Amount:F2}", orderId, amount)
```

### Features
- Compact and readable
- Supports format specifiers with `:` separator
- Supports alignment with `,` separator
- Compatible with capturing hints (`@` and `$`)

## Go Template Syntax

For developers familiar with Go's template system, mtlog also supports double-brace syntax `{{.Property}}`:

```go
logger.Information("User {{.UserId}} logged in at {{.Time}}", userId, loginTime)
```

### Features
- Familiar to Go developers
- Matches Go's standard `text/template` and `html/template` syntax
- Can be mixed with traditional syntax

## Mixed Syntax

You can freely mix both syntaxes in the same template:

```go
logger.Information("User {UserId} ({{.Username}}) logged in from {IP}", userId, username, ipAddress)
```

This is particularly useful when migrating existing code or when different team members have different preferences.

## Format Specifiers

Format specifiers are supported with traditional syntax:

### Numbers
```go
// Zero-padding
logger.Information("Order {Id:000}", 42)           // "042"

// Decimal places
logger.Information("Price: ${Price:F2}", 99.9)     // "$99.90"

// Percentage
logger.Information("CPU: {Usage:P0}", 0.65)        // "65%"
logger.Information("Memory: {Usage:P1}", 0.855)    // "85.5%"
```

### Timestamps (in output templates)
```go
mtlog.WithConsoleTemplate("[${Timestamp:HH:mm:ss} ${Level:u3}] ${Message}")
// Output: [15:04:05 INF] User logged in

mtlog.WithFileTemplate("app.log", "[${Timestamp:yyyy-MM-dd HH:mm:ss.fff}] ${Message}")
// Output: [2024-01-02 15:04:05.123] User logged in
```

### Log Levels (in output templates)
```go
// ${Level:u3} - Three-letter uppercase: INF, WRN, ERR
// ${Level:u}  - Full uppercase: INFORMATION, WARNING, ERROR
// ${Level:l}  - Full lowercase: information, warning, error
```

## Capturing Hints

Both syntaxes support capturing hints:

```go
// @ - Capture complex types
logger.Information("Order {@Order} created", order)
logger.Information("User {{@.User}} updated", user)

// $ - Force scalar/string rendering
logger.Information("Error: {$Error}", complexError)
logger.Information("Details: {{$.Details}}", details)
```

## Examples

### Basic Logging
```go
// Traditional
logger.Information("Hello {Name}!", "World")

// Go template
logger.Information("Hello {{.Name}}!", "World")
```

### With Properties
```go
// Traditional
logger.Information("User {UserId} performed {Action} on {Resource}", 
    userId, action, resource)

// Go template
logger.Information("User {{.UserId}} performed {{.Action}} on {{.Resource}}", 
    userId, action, resource)

// Mixed
logger.Information("User {UserId} ({{.Username}}) performed {Action}", 
    userId, username, action)
```

### Complex Example
```go
type Order struct {
    ID     int
    Total  float64
    Items  int
    Status string
}

order := Order{ID: 123, Total: 99.95, Items: 3, Status: "pending"}

// Traditional with formatting
logger.Information("Order {OrderId:000}: ${Total:F2} for {Items} items - {@Order}", 
    order.ID, order.Total, order.Items, order)

// Go template
logger.Information("Order {{.OrderId}}: ${{.Total}} for {{.Items}} items - {{@.Order}}", 
    order.ID, order.Total, order.Items, order)
```

## Best Practices

1. **Consistency** - Choose one syntax as your primary style within a project
2. **Migration** - When migrating from other logging libraries, use the syntax that requires fewer changes
3. **Team Preference** - Let your team decide which syntax feels more natural
4. **Format Specifiers** - Use traditional syntax when you need format specifiers
5. **Performance** - Both syntaxes have identical performance characteristics

## Performance

Both syntaxes are parsed into the same internal representation and have identical runtime performance. The template parsing is cached, so there's no performance penalty for using either syntax.

## Future Compatibility

Both syntaxes will be maintained and supported going forward. The Go template syntax provides a familiar option for Go developers while the traditional syntax offers compatibility with format specifiers and a more compact notation.