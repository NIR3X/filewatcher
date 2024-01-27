package filewatcher

import (
	"fmt"
	"testing"
	"time"
)

func TestFileWatcher(t *testing.T) {
	f := NewFileWatcher(time.Second, func(path string, isDir bool) {
		if isDir {
			fmt.Println("created dir:", path)
		} else {
			fmt.Println("created file:", path)
		}
	}, func(path string, isDir bool) {
		if isDir {
			fmt.Println("removed dir:", path)
		} else {
			fmt.Println("removed file:", path)
		}
	}, func(path string, isDir bool) {
		if isDir {
			fmt.Println("modified dir:", path)
		} else {
			fmt.Println("modified file:", path)
		}
	})
	defer f.Close()

	err := f.Watch("a.txt")
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Minute)
}
