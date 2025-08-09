package mtlog005

import (
	"time"

	"github.com/willibrandon/mtlog"
)

// User demonstrates a struct with sensitive fields
type User struct {
	ID        int
	Username  string
	Email     string
	Password  string // Sensitive field
	APIKey    string // Sensitive field
	LastLogin time.Time
}

// Order demonstrates a struct without sensitive fields
type Order struct {
	ID         int
	CustomerID int
	Total      float64
	Status     string
	CreatedAt  time.Time
}

// Account demonstrates a struct with mixed fields
type Account struct {
	AccountNumber string
	Balance       float64
	Token         string // Sensitive field
	CreditCard    string // Sensitive field
	Owner         string
}

func TestLogValueStubGeneration() {
	log := mtlog.New()

	// These should trigger MTLOG005 with LogValue() stub suggestion
	user := User{
		ID:        123,
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "secret123",
		APIKey:    "sk_live_abc123",
		LastLogin: time.Now(),
	}
	log.Information("User logged in: {User}", user) // want `\[MTLOG005\] suggestion: consider using @ prefix for complex type.*User to enable capturing`

	order := Order{
		ID:         456,
		CustomerID: 123,
		Total:      99.99,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	log.Information("Order created: {Order}", order) // want `\[MTLOG005\] suggestion: consider using @ prefix for complex type.*Order to enable capturing`

	account := Account{
		AccountNumber: "ACC-123456",
		Balance:       1000.00,
		Token:         "bearer_token_xyz",
		CreditCard:    "4111111111111111",
		Owner:         "Alice Smith",
	}
	log.Information("Account details: {Account}", account) // want `\[MTLOG005\] suggestion: consider using @ prefix for complex type.*Account to enable capturing`

	// Pointer types should also work
	log.Information("User pointer: {UserPtr}", &user) // want `\[MTLOG005\] suggestion: consider using @ prefix for complex type.*User to enable capturing`
}