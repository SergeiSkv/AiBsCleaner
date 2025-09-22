package test

import "fmt"

func SimpleConcat() {
	// First loop - should be detected
	result := ""
	for i := 0; i < 10; i++ {
		result += fmt.Sprintf("item%d", i)
	}

	// Second loop - should also be detected
	result2 := ""
	for j := 0; j < 10; j++ {
		result2 += fmt.Sprintf("item%d", j)
	}

	// Third loop - should also be detected
	s := ""
	for k := 0; k < 10; k++ {
		s += "test"
	}

	fmt.Println(result, result2, s)
}
