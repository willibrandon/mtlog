// Package main demonstrates using ForType for automatic SourceContext from type names.
package main

import (
	"fmt"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// Domain types for the example
type User struct {
	ID    int
	Name  string
	Email string
}

type Product struct {
	SKU         string
	Name        string
	Price       float64
	CategoryID  int
}

type Order struct {
	ID       string
	UserID   int
	Products []Product
	Total    float64
}

// Service types
type UserService struct {
	logger core.Logger
}

type ProductService struct {
	logger core.Logger
}

type OrderService struct {
	logger core.Logger
}

func main() {
	// Create a logger with console output that shows properties
	logger := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithMinimumLevel(core.DebugLevel),
	)

	fmt.Println("=== ForType Example ===")
	fmt.Println()

	// Demonstrate basic ForType usage
	demonstrateBasicForType(logger)

	fmt.Println()
	fmt.Println("=== Service-based Logging ===")
	fmt.Println()

	// Demonstrate service-based logging
	demonstrateServiceLogging(logger)

	fmt.Println()
	fmt.Println("=== Comparison with ForContext ===")
	fmt.Println()

	// Show comparison with manual SourceContext
	demonstrateComparison(logger)

	fmt.Println()
	fmt.Println("=== Method Chaining Example ===")
	fmt.Println()

	// Demonstrate method chaining
	demonstrateChaining(logger)

	fmt.Println()
	fmt.Println("=== TypeNameOptions Example ===")
	fmt.Println()

	// Demonstrate TypeNameOptions usage
	demonstrateTypeNameOptions(logger)
}

func demonstrateBasicForType(logger core.Logger) {
	// Using ForType for different domain entities
	
	// User operations
	userLogger := mtlog.ForType[User](logger)
	userLogger.Information("User {UserId} created with email {Email}", 123, "john@example.com")
	userLogger.Warning("User {UserId} attempted invalid login", 123)

	// Product operations  
	productLogger := mtlog.ForType[Product](logger)
	productLogger.Information("Product {SKU} added to catalog with price ${Price:F2}", "LAPTOP-001", 999.99)
	productLogger.Debug("Product {SKU} inventory updated", "LAPTOP-001")

	// Order operations
	orderLogger := mtlog.ForType[Order](logger)
	orderLogger.Information("Order {OrderId} created for user {UserId} with total ${Total:F2}", "ORD-456", 123, 1999.98)
	orderLogger.Error("Order {OrderId} payment failed: {Error}", "ORD-456", "Credit card declined")
}

func demonstrateServiceLogging(logger core.Logger) {
	// Create services with type-specific loggers
	userService := &UserService{
		logger: mtlog.ForType[UserService](logger),
	}

	productService := &ProductService{
		logger: mtlog.ForType[ProductService](logger),
	}

	orderService := &OrderService{
		logger: mtlog.ForType[OrderService](logger),
	}

	// Simulate service operations
	userService.CreateUser("John Doe", "john@example.com")
	productService.AddProduct("LAPTOP-001", "Gaming Laptop", 999.99)
	orderService.ProcessOrder("ORD-789", 123, []string{"LAPTOP-001"})
}

func (us *UserService) CreateUser(name, email string) {
	us.logger.Debug("Starting user creation process")
	us.logger.Information("Creating user {Name} with email {Email}", name, email)
	
	// Simulate validation
	if email == "" {
		us.logger.Error("User creation failed: email is required")
		return
	}
	
	us.logger.Information("User {Name} created successfully", name)
}

func (ps *ProductService) AddProduct(sku, name string, price float64) {
	ps.logger.Debug("Adding product to catalog")
	ps.logger.Information("Adding product {SKU}: {Name} at price ${Price:F2}", sku, name, price)
	
	// Simulate business logic
	if price <= 0 {
		ps.logger.Error("Product addition failed: price must be positive")
		return
	}
	
	ps.logger.Information("Product {SKU} added successfully", sku)
}

func (os *OrderService) ProcessOrder(orderID string, userID int, productSKUs []string) {
	os.logger.Information("Processing order {OrderId} for user {UserId}", orderID, userID)
	os.logger.Debug("Order contains {ProductCount} products", len(productSKUs))
	
	// Simulate order processing
	for _, sku := range productSKUs {
		os.logger.Debug("Processing product {SKU} in order {OrderId}", sku, orderID)
	}
	
	os.logger.Information("Order {OrderId} processed successfully", orderID)
}

func demonstrateComparison(logger core.Logger) {
	fmt.Println("Using ForType[User]():")
	
	// Using ForType - automatic and type-safe
	mtlog.ForType[User](logger).Information("User operation completed")
	
	fmt.Println()
	fmt.Println("Using ForContext(\\\"SourceContext\\\", \\\"User\\\"):")
	
	// Using manual SourceContext - more verbose and error-prone
	logger.ForContext("SourceContext", "User").Information("User operation completed")
	
	fmt.Println()
	fmt.Println("Both produce identical SourceContext values!")
	
	// Demonstrate pointer dereferencing
	fmt.Println()
	fmt.Println("ForType with pointers:")
	mtlog.ForType[*User](logger).Information("Pointer type automatically dereferenced")
	mtlog.ForType[**User](logger).Information("Double pointer also dereferenced")
}

// Demonstrate chaining ForType with other logger methods
func demonstrateChaining(logger core.Logger) {
	// ForType can be chained with other logger methods
	mtlog.ForType[User](logger).
		ForContext("Operation", "Registration").
		ForContext("Source", "WebAPI").
		Information("User registration process started")

	// This creates a logger with:
	// - SourceContext: "User"  
	// - Operation: "Registration"
	// - Source: "WebAPI"
}

func demonstrateTypeNameOptions(logger core.Logger) {
	fmt.Println("ForType uses simple type names by default, but you can customize this behavior")
	fmt.Println("using extractTypeName with TypeNameOptions for more control:")
	fmt.Println()

	// Show the default behavior that ForType uses
	fmt.Println("Default ForType behavior (simple names):")
	mtlog.ForType[User](logger).Information("Simple type name used")
	mtlog.ForType[ProductService](logger).Information("Service type with simple name")
	
	fmt.Println()
	fmt.Println("Custom logger examples using extractTypeName with TypeNameOptions:")
	
	// Example 1: Include package path for disambiguation
	opts1 := mtlog.TypeNameOptions{IncludePackage: true, PackageDepth: 1}
	name1 := mtlog.ExtractTypeName[User](opts1)
	logger.ForContext("SourceContext", name1).Information("Custom logger with package: {TypeName}", name1)
	
	// Example 2: Add prefix for microservice identification
	opts2 := mtlog.TypeNameOptions{Prefix: "UserAPI."}
	name2 := mtlog.ExtractTypeName[User](opts2)
	logger.ForContext("SourceContext", name2).Information("Custom logger with prefix: {TypeName}", name2)
	
	// Example 3: Add suffix for component type
	opts3 := mtlog.TypeNameOptions{Suffix: ".Service"}
	name3 := mtlog.ExtractTypeName[UserService](opts3)
	logger.ForContext("SourceContext", name3).Information("Custom logger with suffix: {TypeName}", name3)
	
	// Example 4: Combine multiple options
	opts4 := mtlog.TypeNameOptions{
		IncludePackage: true,
		PackageDepth:   1,
		Prefix:         "MyApp.",
		Suffix:         ".Handler",
	}
	name4 := mtlog.ExtractTypeName[User](opts4)
	logger.ForContext("SourceContext", name4).Information("Complex custom logger: {TypeName}", name4)
	
	fmt.Println()
	fmt.Println("You can create helper functions for consistent custom loggers:")
	fmt.Println()
	
	// Demonstrate helper function approach
	customUserLogger := createCustomLogger[User](logger, mtlog.TypeNameOptions{Prefix: "WebAPI."})
	customUserLogger.Information("Using helper function for consistent naming")
	
	fmt.Println()
	// Example 5: SimplifyAnonymous for cleaner anonymous struct names
	opts5 := mtlog.TypeNameOptions{SimplifyAnonymous: true}
	name5 := mtlog.ExtractTypeName[struct{ Name string }](opts5)
	logger.ForContext("SourceContext", name5).Information("Anonymous struct with simplified name: {TypeName}", name5)
	
	fmt.Println()
	fmt.Println("Common scenarios for TypeNameOptions:")
	fmt.Println("- Monorepos with multiple User types: Use IncludePackage=true")
	fmt.Println("- Microservices: Add Prefix like 'UserService.' or 'PaymentAPI.'")  
	fmt.Println("- Clean separation: Add Suffix like '.Handler' or '.Repository'")
	fmt.Println("- Anonymous structs: Use SimplifyAnonymous=true for cleaner names")
	fmt.Println("- Complex projects: Combine options for hierarchical naming")
}

// Helper function to create custom loggers with TypeNameOptions
func createCustomLogger[T any](baseLogger core.Logger, opts mtlog.TypeNameOptions) core.Logger {
	typeName := mtlog.ExtractTypeName[T](opts)
	return baseLogger.ForContext("SourceContext", typeName)
}
