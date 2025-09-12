package test

import (
	"fmt"
	"os"
)

func TestIgnoreExample() {
	// This should be detected
	result := ""
	for i := 0; i < 1000; i++ {
		result += fmt.Sprintf("item%d", i) // STRING_CONCAT_IN_LOOP
	}

	// Test with abc:ignore comment
	result2 := ""
	// abc:ignore STRING_CONCAT_IN_LOOP
	for j := 0; j < 1000; j++ {
		result2 += fmt.Sprintf("item%d", j) // This should be ignored
	}

	// Test defer in loop
	//nolint:gocritic // intentionally demonstrates defer-in-loop pattern for analyzer tests
	for i := 0; i < 100; i++ {
		file, _ := os.Open("test.txt")
		defer func() {
			_ = file.Close()
		}()
	}

	// Test with abc:ignore for defer
	// abc:ignore DEFER_IN_LOOP
	for i := 0; i < 100; i++ {
		file2, _ := os.Open("test2.txt")
		_ = file2.Close() // This should be ignored
	}
}
