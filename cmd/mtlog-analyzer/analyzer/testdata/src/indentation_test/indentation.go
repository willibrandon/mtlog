package indentation_test

import "github.com/willibrandon/mtlog"

func testIndentation() {
	log := mtlog.New()
	
	// Test case 1: No indentation
	log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
	
	// Test case 2: Single tab indentation
	log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
	
	// Test case 3: Two tabs indentation
		log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
		
	// Test case 4: Three tabs indentation
			log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
			
	// Test case 5: Mixed spaces and tabs (column 10)
	  log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
	  
	// Test case 6: Deep nesting (4 tabs)
				log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
				
	// Test case 7: Inside if statement
	if true {
		log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
	}
	
	// Test case 8: Inside nested blocks
	if true {
		for i := 0; i < 10; i++ {
			log.Information("Test {A} {B}") // existing comment // want `\[MTLOG001\] template has 2 properties but 0 arguments provided`
		}
	}
}