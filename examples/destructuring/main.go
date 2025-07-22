package main

import (
	"time"
	
	"github.com/willibrandon/mtlog"
)

type User struct {
	ID        int
	Username  string
	Email     string       `log:"email"`
	Password  string       `log:"-"` // Exclude from logs
	CreatedAt time.Time
	Profile   UserProfile
	Tags      []string
	Settings  map[string]interface{}
}

type UserProfile struct {
	FirstName string
	LastName  string
	Age       int
	Address   Address
}

type Address struct {
	Street  string
	City    string
	Country string
	ZipCode string
}

func main() {
	// Example 1: Basic destructuring
	log1 := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithDestructuring(),
	)
	
	user := User{
		ID:        123,
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret123", // This won't be logged
		CreatedAt: time.Now(),
		Profile: UserProfile{
			FirstName: "Alice",
			LastName:  "Smith",
			Age:       28,
			Address: Address{
				Street:  "123 Main St",
				City:    "Anytown",
				Country: "USA",
				ZipCode: "12345",
			},
		},
		Tags:     []string{"premium", "verified", "developer"},
		Settings: map[string]interface{}{
			"theme":         "dark",
			"notifications": true,
			"language":      "en",
		},
	}
	
	log1.Information("User logged in: {@User}", user)
	
	// Example 2: Destructuring with limits
	log2 := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithCustomDestructuring(2, 50, 5), // Max depth 2, strings truncated at 50 chars, max 5 items in collections
	)
	
	// Create a large dataset
	largeData := struct {
		LongText string
		Numbers  []int
		Nested   map[string]interface{}
	}{
		LongText: "This is a very long text that should be truncated because it exceeds the maximum string length limit",
		Numbers:  []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Will be truncated to 5 items
		Nested: map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "This won't be fully destructured due to depth limit",
				},
			},
		},
	}
	
	log2.Information("Large data: {@Data}", largeData)
	
	// Example 3: Without destructuring (default behavior)
	log3 := mtlog.New(
		mtlog.WithConsoleProperties(),
		// No destructuring - complex objects will use default Go formatting
	)
	
	log3.Information("User without destructuring: {@User}", user)
	
	// Example 4: Destructuring with errors and special types
	log4 := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithDestructuring(),
	)
	
	type Response struct {
		StatusCode int
		Headers    map[string]string
		Body       []byte
		Error      error
		Duration   time.Duration
		Timestamp  time.Time
	}
	
	resp := Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Request-ID": "abc123",
		},
		Body:      []byte(`{"status": "ok"}`),
		Error:     nil,
		Duration:  150 * time.Millisecond,
		Timestamp: time.Now(),
	}
	
	log4.Information("API response: {@Response}", resp)
	
	// Example 5: Circular references (destructurer should handle gracefully)
	type Node struct {
		Value int
		Next  *Node
	}
	
	// Create a simple linked list
	node1 := &Node{Value: 1}
	node2 := &Node{Value: 2}
	node3 := &Node{Value: 3}
	node1.Next = node2
	node2.Next = node3
	// node3.Next = node1 // Uncomment for circular reference
	
	log4.Information("Linked list: {@List}", node1)
	
	// Example 6: Interface values and nil handling
	var items []interface{}
	items = append(items, "string", 123, true, nil, 3.14)
	items = append(items, map[string]int{"a": 1, "b": 2})
	items = append(items, struct{ Name string }{"test"})
	
	log4.Information("Mixed types: {@Items}", items)
}