package mtlog001

import "github.com/willibrandon/mtlog"

func testTemplateMismatchFixes() {
	log := mtlog.New()

	// Too few arguments - should suggest adding placeholders
	log.Information("User {UserId} performed {Action} on {Resource}") // want `\[MTLOG001\] template has 3 properties but 0 arguments provided`
	
	// Too few arguments with existing comment - TODO should go on next line
	log.Information("Failed {Operation}") // existing comment // want `\[MTLOG001\] template has 1 properties but 0 arguments provided`

	// Too many arguments - should suggest removing extras
	log.Information("Hello {Name}", "Alice", "Bob", "Charlie") // want `\[MTLOG001\] template has 1 properties but 3 arguments provided`

	// Correct number of arguments - no fix needed
	log.Information("User {UserId} logged in", 123)
}