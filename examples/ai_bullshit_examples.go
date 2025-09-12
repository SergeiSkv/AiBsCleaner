package examples

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// AI создает Factory для простого сложения - bullshit!
type NumberAdderFactory interface {
	CreateNumberAdder() NumberAdderStrategy
}

type NumberAdderStrategy interface {
	AddNumbers(a, b int) int
}

type SimpleNumberAdderFactory struct{}

func (f *SimpleNumberAdderFactory) CreateNumberAdder() NumberAdderStrategy {
	return &SimpleNumberAdderStrategy{}
}

type SimpleNumberAdderStrategy struct{}

// AI bullshit: 20 строк кода для сложения двух чисел с goroutines!
func (s *SimpleNumberAdderStrategy) AddNumbers(a, b int) int {
	// Initialize result channel for parallel processing
	resultCh := make(chan int, 1)

	// Create goroutine for mathematical operation
	go func() {
		// Use reflection for type safety (AI bullshit!)
		aVal := reflect.ValueOf(a)
		bVal := reflect.ValueOf(b)

		// Perform the addition with context
		ctx := context.Background()
		result := s.performAdditionWithContext(ctx, int(aVal.Int()), int(bVal.Int()))

		// Send result through channel
		resultCh <- result
	}()

	// Wait for result from goroutine
	return <-resultCh
}

func (s *SimpleNumberAdderStrategy) performAdditionWithContext(ctx context.Context, a, b int) int {
	// Add artificial delay to simulate complex operation
	select {
	case <-time.After(1 * time.Millisecond):
		return a + b
	case <-ctx.Done():
		return 0
	}
}

// AI создает Manager для всего
type ConfigurationManager interface {
	GetConfiguration() Configuration
}

type Configuration struct {
	Value string
}

type SimpleConfigurationManager struct {
	config Configuration
}

func (m *SimpleConfigurationManager) GetConfiguration() Configuration {
	return m.config
}

// AI создает Handler для простого вывода
type OutputHandler interface {
	HandleOutput(message string)
}

type ConsoleOutputHandler struct{}

func (h *ConsoleOutputHandler) HandleOutput(message string) {
	fmt.Println(message)
}

// AI создает Service для проверки четности - 30 строк для n%2!
type NumberValidationService interface {
	ValidateEvenness(number int) bool
}

type DefaultNumberValidationService struct {
	logger OutputHandler
}

func (s *DefaultNumberValidationService) ValidateEvenness(number int) bool {
	// Initialize validation context
	ctx := context.Background()

	// Create validation result channel
	resultCh := make(chan bool, 1)

	// Spawn validation goroutine
	go func() {
		// Log validation start
		s.logger.HandleOutput("Starting evenness validation")

		// Use reflection to get number value
		numVal := reflect.ValueOf(number)
		actualNumber := int(numVal.Int())

		// Perform complex validation logic
		isEven := s.performEvennessCheck(ctx, actualNumber)

		// Log validation complete
		s.logger.HandleOutput("Evenness validation complete")

		// Send result
		resultCh <- isEven
	}()

	// Wait for validation result
	return <-resultCh
}

func (s *DefaultNumberValidationService) performEvennessCheck(ctx context.Context, number int) bool {
	// Add timeout for validation
	select {
	case <-time.After(100 * time.Microsecond):
		// Complex mathematical operation (AI thinks this is needed)
		remainder := number % 2
		return remainder == 0
	case <-ctx.Done():
		return false
	}
}

// AI создает Provider даже для констант
type ConstantProvider interface {
	ProvideConstant() int
}

type MagicNumberProvider struct{}

func (p *MagicNumberProvider) ProvideConstant() int {
	// Use reflection to create the number (AI bullshit)
	val := reflect.ValueOf(42)
	return int(val.Int())
}
