package mtlog

// Repository interface for testing - used across multiple test files
type Repository interface {
	Save(interface{}) error
}

// UserRepository implements Repository for testing
type UserRepository struct{}

func (ur *UserRepository) Save(interface{}) error { return nil }