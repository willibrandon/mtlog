package testutil

import (
	"slices"
	"testing"
	"time"
)

// Eventually waits for a condition to be true within the timeout period.
// It checks the condition every 10ms until it returns true or the timeout expires.
func Eventually(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	if message != "" {
		t.Fatal(message)
	} else {
		t.Fatal("Condition not met within timeout")
	}
}

// EventuallyEqual waits for a function to return the expected value.
func EventuallyEqual[T comparable](t *testing.T, getter func() T, expected T, timeout time.Duration) {
	t.Helper()

	Eventually(t, func() bool {
		return getter() == expected
	}, timeout, "")
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		if message != "" {
			t.Fatalf("%s: %v", message, err)
		} else {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		if message != "" {
			t.Fatal(message)
		} else {
			t.Fatal("Expected error but got nil")
		}
	}
}

// AssertEqual fails the test if actual != expected.
func AssertEqual[T comparable](t *testing.T, actual, expected T, message string) {
	t.Helper()
	if actual != expected {
		if message != "" {
			t.Fatalf("%s: expected %v, got %v", message, expected, actual)
		} else {
			t.Fatalf("Expected %v, got %v", expected, actual)
		}
	}
}

// AssertContains fails the test if the slice doesn't contain the value.
func AssertContains[T comparable](t *testing.T, slice []T, value T, message string) {
	t.Helper()
	if slices.Contains(slice, value) {
		return
	}
	if message != "" {
		t.Fatalf("%s: %v not found in slice", message, value)
	} else {
		t.Fatalf("%v not found in slice", value)
	}
}
