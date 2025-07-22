package destructure

import (
	"testing"
	"time"
)

type BenchUser struct {
	ID        int
	Username  string
	Email     string
	CreatedAt time.Time
	Profile   BenchProfile
	Tags      []string
	Settings  map[string]interface{}
}

type BenchProfile struct {
	FirstName string
	LastName  string
	Age       int
	Address   BenchAddress
}

type BenchAddress struct {
	Street  string
	City    string
	Country string
	ZipCode string
}

func createBenchUser() BenchUser {
	return BenchUser{
		ID:        123,
		Username:  "testuser",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		Profile: BenchProfile{
			FirstName: "Test",
			LastName:  "User",
			Age:       30,
			Address: BenchAddress{
				Street:  "123 Test St",
				City:    "Test City",
				Country: "Test Country",
				ZipCode: "12345",
			},
		},
		Tags:     []string{"tag1", "tag2", "tag3"},
		Settings: map[string]interface{}{
			"theme":    "dark",
			"language": "en",
			"notifications": map[string]bool{
				"email": true,
				"sms":   false,
			},
		},
	}
}

func BenchmarkDefaultDestructurer(b *testing.B) {
	d := NewDefaultDestructurer()
	factory := &mockPropertyFactory{}
	user := createBenchUser()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		d.TryDestructure(user, factory)
	}
}

func BenchmarkCachedDestructurer(b *testing.B) {
	d := NewCachedDestructurer()
	factory := &mockPropertyFactory{}
	user := createBenchUser()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		d.TryDestructure(user, factory)
	}
}

func BenchmarkCachedDestructurerParallel(b *testing.B) {
	d := NewCachedDestructurer()
	factory := &mockPropertyFactory{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		user := createBenchUser()
		for pb.Next() {
			d.TryDestructure(user, factory)
		}
	})
}

// Benchmark different struct sizes
func BenchmarkDestructurerBySize(b *testing.B) {
	// Small struct
	type Small struct {
		ID   int
		Name string
	}
	
	// Medium struct
	type Medium struct {
		ID       int
		Name     string
		Email    string
		Phone    string
		Address  string
		City     string
		Country  string
		ZipCode  string
		Active   bool
		Created  time.Time
	}
	
	// Large struct
	type Large struct {
		Field1  string
		Field2  string
		Field3  string
		Field4  string
		Field5  string
		Field6  string
		Field7  string
		Field8  string
		Field9  string
		Field10 string
		Field11 int
		Field12 int
		Field13 int
		Field14 int
		Field15 int
		Field16 bool
		Field17 bool
		Field18 bool
		Field19 time.Time
		Field20 time.Time
	}
	
	small := Small{ID: 1, Name: "test"}
	medium := Medium{
		ID: 1, Name: "test", Email: "test@example.com",
		Phone: "123-456-7890", Address: "123 Test St",
		City: "Test City", Country: "Test Country",
		ZipCode: "12345", Active: true, Created: time.Now(),
	}
	large := Large{
		Field1: "1", Field2: "2", Field3: "3", Field4: "4", Field5: "5",
		Field6: "6", Field7: "7", Field8: "8", Field9: "9", Field10: "10",
		Field11: 11, Field12: 12, Field13: 13, Field14: 14, Field15: 15,
		Field16: true, Field17: false, Field18: true,
		Field19: time.Now(), Field20: time.Now(),
	}
	
	factory := &mockPropertyFactory{}
	
	b.Run("Small-Default", func(b *testing.B) {
		d := NewDefaultDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(small, factory)
		}
	})
	
	b.Run("Small-Cached", func(b *testing.B) {
		d := NewCachedDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(small, factory)
		}
	})
	
	b.Run("Medium-Default", func(b *testing.B) {
		d := NewDefaultDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(medium, factory)
		}
	})
	
	b.Run("Medium-Cached", func(b *testing.B) {
		d := NewCachedDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(medium, factory)
		}
	})
	
	b.Run("Large-Default", func(b *testing.B) {
		d := NewDefaultDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(large, factory)
		}
	})
	
	b.Run("Large-Cached", func(b *testing.B) {
		d := NewCachedDestructurer()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			d.TryDestructure(large, factory)
		}
	})
}