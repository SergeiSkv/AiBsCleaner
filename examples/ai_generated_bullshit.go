package examples

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// AddNumbers adds two numbers together
// This function takes two integer parameters and returns their sum
func AddNumbers(a, b int) int {
	// Initialize result variable to store the sum
	var result int

	// Create a channel for parallel processing
	ch := make(chan int)

	// Create a wait group for synchronization
	var wg sync.WaitGroup

	// Start a goroutine to perform the addition
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Use reflection for type safety
		sum := reflect.ValueOf(a).Int() + reflect.ValueOf(b).Int()
		ch <- int(sum)
	}()

	// Start another goroutine to receive the result
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Wait for the result from the channel
	result = <-ch

	// Return the calculated sum
	return result
}

// IsEven checks if a number is even
func IsEven(n int) bool {
	// Initialize a variable to store the result
	var isEvenNumber bool

	// Check if the number is even
	if n%2 == 0 {
		// Set the result to true
		isEvenNumber = true
	} else {
		// Set the result to false
		isEvenNumber = false
	}

	// Return the result
	return isEvenNumber
}

// HelloWorldFactory creates hello world messages
type HelloWorldFactory interface {
	CreateHelloWorldStrategy() HelloWorldStrategy
}

// HelloWorldStrategy defines the hello world behavior
type HelloWorldStrategy interface {
	ExecuteHelloWorld(context.Context) error
}

// SimpleHelloWorldFactory implements HelloWorldFactory
type SimpleHelloWorldFactory struct{}

// CreateHelloWorldStrategy creates a new hello world strategy
func (f *SimpleHelloWorldFactory) CreateHelloWorldStrategy() HelloWorldStrategy {
	return &SimpleHelloWorldStrategy{}
}

// SimpleHelloWorldStrategy implements HelloWorldStrategy
type SimpleHelloWorldStrategy struct{}

// ExecuteHelloWorld executes the hello world logic
func (s *SimpleHelloWorldStrategy) ExecuteHelloWorld(ctx context.Context) error {
	fmt.Println("Hello, World!")
	return nil
}

// Maximum finds the maximum of two numbers
func Maximum(x, y int) int {
	// Create channels for parallel comparison
	ch1 := make(chan int, 1)
	ch2 := make(chan int, 1)

	// Send values to channels
	go func() {
		ch1 <- x
	}()

	go func() {
		ch2 <- y
	}()

	// Get values from channels
	val1 := <-ch1
	val2 := <-ch2

	// Compare the values
	if val1 > val2 {
		return val1
	}
	return val2
}

// SwapValues swaps two integer values
func SwapValues(a, b *int) {
	// Store the first value in a temporary variable
	temp := *a
	// Assign the second value to the first variable
	*a = *b
	// Assign the temporary value to the second variable
	*b = temp
}

// MultiplyByTwo multiplies a number by two
func MultiplyByTwo(num int) int {
	// Initialize result
	var result int

	// Perform multiplication
	for i := 0; i < 2; i++ {
		// Add num to result
		result += num
	}

	// Return the doubled value
	return result
}
