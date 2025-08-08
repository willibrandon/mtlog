package mtlog002

import "github.com/willibrandon/mtlog"

func testInvalidFormatSpecifiers() {
	log := mtlog.New()
	
	// Integer format errors - common .NET style
	log.Information("Count: {Count:d}", 42) // want "invalid format specifier"
	log.Information("Count: {Count:d3}", 42) // want "invalid format specifier"
	log.Information("Count: {Count:D}", 42) // want "invalid format specifier"
	log.Information("Count: {Count:decimal}", 42) // want "invalid format specifier"
	log.Information("Count: {Count:int}", 42) // want "invalid format specifier"
	log.Information("Count: {Count:3}", 42) // want "invalid format specifier"
	
	// Float format errors
	log.Information("Price: {Price:f}", 19.99) // want "invalid format specifier"
	log.Information("Price: {Price:f3}", 19.99) // want "invalid format specifier"
	log.Information("Price: {Price:float}", 19.99) // want "invalid format specifier"
	log.Information("Price: {Price:currency}", 19.99) // want "invalid format specifier"
	log.Information("Price: {Price:c}", 19.99) // want "invalid format specifier"
	log.Information("Price: {Price:n}", 19.99) // want "invalid format specifier"
	
	// Percentage format errors
	log.Information("Usage: {Usage:p}", 0.85) // want "invalid format specifier"
	log.Information("Usage: {Usage:p1}", 0.85) // want "invalid format specifier"
	log.Information("Usage: {Usage:percent}", 0.85) // want "invalid format specifier"
	log.Information("Usage: {Usage:percentage}", 0.85) // want "invalid format specifier"
	
	// Exponential format errors
	log.Information("Value: {Value:e}", 1.23e10) // want "invalid format specifier"
	log.Information("Value: {Value:e2}", 1.23e10) // want "invalid format specifier"
	log.Information("Value: {Value:exp}", 1.23e10) // want "invalid format specifier"
	log.Information("Value: {Value:exponential}", 1.23e10) // want "invalid format specifier"
	
	// General format errors
	log.Information("Result: {Result:g}", 123.456) // want "invalid format specifier"
	log.Information("Result: {Result:g2}", 123.456) // want "invalid format specifier"
	log.Information("Result: {Result:general}", 123.456) // want "invalid format specifier"
	log.Information("Result: {Result:r}", 123.456) // want "invalid format specifier"
	log.Information("Result: {Result:roundtrip}", 123.456) // want "invalid format specifier"
	
	// Hex format errors
	log.Information("Code: {Code:h}", 255) // want "invalid format specifier"
	log.Information("Code: {Code:h8}", 255) // want "invalid format specifier"
	log.Information("Code: {Code:hex}", 255) // want "invalid format specifier"
	log.Information("Code: {Code:hexadecimal}", 255) // want "invalid format specifier"
	
	// These should not trigger errors (valid formats)
	log.Information("Valid: {Count:000}", 42)
	log.Information("Valid: {Price:F2}", 19.99)
	log.Information("Valid: {Usage:P1}", 0.85)
	log.Information("Valid: {Value:E2}", 1.23e10)
	log.Information("Valid: {Result:G2}", 123.456)
	log.Information("Valid: {Code:X8}", 255)
	log.Information("Valid: {Code:x4}", 255)
}