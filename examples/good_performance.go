package examples

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	compiledRegex = regexp.MustCompile(`\d+`)
)

func GoodStringConcatenation() {
	var builder strings.Builder
	builder.Grow(6000) // Pre-allocate approximate size
	for i := 0; i < 1000; i++ {
		builder.WriteString(fmt.Sprintf("Item %d ", i))
	}
	result := builder.String()
	_ = result
}

func GoodDeferHandling() {
	for i := 0; i < 100; i++ {
		func(idx int) {
			file := openFile(idx)
			defer file.Close()
			// Process file
		}(i)
	}
}

func GoodSliceAppend() {
	items := make([]int, 0, 10000) // Pre-allocate capacity
	for i := 0; i < 10000; i++ {
		items = append(items, i)
	}
}

func GoodLoopAllocation() {
	buffer := make([]byte, 1024) // Allocate once
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			// Reuse buffer
			_ = buffer
		}
	}
}

func GoodRegexUsage() {
	for i := 0; i < 1000; i++ {
		_ = compiledRegex.MatchString("test123")
	}
}

func GoodGoroutinePool() {
	const maxWorkers = 10
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer func() {
				<-sem
				wg.Done()
			}()
			fmt.Println(idx)
		}(i) // Pass i as parameter
	}
	wg.Wait()
}

func GoodTimeUsage() {
	now := time.Now() // Cache time outside loop
	formatted := now.Format("2006-01-02 15:04:05")
	for i := 0; i < 1000; i++ {
		_ = formatted
	}
}

func GoodMapWithSize() {
	m := make(map[string]int, 10000) // Pre-allocate size
	for i := 0; i < 10000; i++ {
		m[fmt.Sprintf("key%d", i)] = i
	}
}

func GoodStringIteration() {
	str := "Hello, World! This is a test string"
	bytes := []byte(str) // Convert once if byte access is needed
	for i := 0; i < len(bytes); i++ {
		_ = bytes[i]
	}
}

func GoodBufferedChannel() {
	ch := make(chan int, 100) // Buffered channel
	go func() {
		for i := 0; i < 100; i++ {
			ch <- i
		}
		close(ch)
	}()
}