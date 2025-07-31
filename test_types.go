package mtlog

// Repository interface for testing - used across multiple test files
type Repository interface {
	Save(any) error
}

// UserRepository implements Repository for testing
type UserRepository struct{}

func (ur *UserRepository) Save(any) error { return nil }
