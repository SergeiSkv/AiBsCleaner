package examples

import (
	"fmt"
	"regexp"
	"time"
)

func BadStringConcatenation() {
	var result string
	for i := 0; i < 1000; i++ {
		result += fmt.Sprintf("Item %d ", i)
	}
}

func BadDeferInLoop() {
	for i := 0; i < 100; i++ {
		file := openFile(i)
		if file != nil {
			defer file.Close()
		}
	}
}

func BadSliceAppend() {
	var items []int
	for i := 0; i < 10000; i++ {
		items = append(items, i)
	}
}

func BadNestedLoopAllocation() {
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			data := make([]byte, 1024)
			_ = data
		}
	}
}

func BadRegexCompile() {
	for i := 0; i < 1000; i++ {
		re := regexp.MustCompile(`\d+`)
		_ = re.MatchString("test123")
	}
}

func BadGoroutineLoop() {
	for i := 0; i < 10000; i++ {
		go func() {
			fmt.Println(i)
		}()
	}
}

func BadTimeInLoop() {
	for i := 0; i < 1000; i++ {
		now := time.Now()
		_ = now.Format("2006-01-02 15:04:05")
	}
}

func BadMapWithoutSize() {
	m := make(map[string]int)
	for i := 0; i < 10000; i++ {
		m[fmt.Sprintf("key%d", i)] = i
	}
}

func BadRangeOverString() {
	str := "Hello, World! This is a test string"
	for _, r := range str {
		_ = r
	}
}

func BadUnbufferedChannel() {
	ch := make(chan int)
	go func() {
		for i := 0; i < 100; i++ {
			ch <- i
		}
	}()
}

func openFile(i int) interface{ Close() error } {
	return &mockFile{}
}

type mockFile struct{}

func (m *mockFile) Close() error {
	return nil
}
