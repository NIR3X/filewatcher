# FileWatcher - Go Package for File and Directory Watching

FileWatcher is a Go package that provides a file and directory watcher with callbacks for create, remove, and modify events.

## Installation

To use FileWatcher in your Go project, you can install it using `go get`:

```bash
go get -u github.com/NIR3X/filewatcher
```

## Usage

```go
package main

import (
	"fmt"
	"time"
	"github.com/NIR3X/filewatcher"
)

func main() {
	// Initialize FileWatcher
	f := filewatcher.NewFileWatcher(time.Second, func(path string, isDir bool) {
		// Callback for created files and directories
		if isDir {
			fmt.Println("created dir:", path)
		} else {
			fmt.Println("created file:", path)
		}
	}, func(path string, isDir bool) {
		// Callback for removed files and directories
		if isDir {
			fmt.Println("removed dir:", path)
		} else {
			fmt.Println("removed file:", path)
		}
	}, func(path string, isDir bool) {
		// Callback for modified files and directories
		if isDir {
			fmt.Println("modified dir:", path)
		} else {
			fmt.Println("modified file:", path)
		}
	})
	defer f.Close()

	// Watch the current directory
	err := f.Watch(".")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Keep the application running to receive file events
	select {}
}
```

## API Documentation

* `NewFileWatcher`: Create a new instance of the FileWatcher.
* `Close`: Close the FileWatcher and stop watching.
* `Watch`: Start watching a specified file or directory.
* `Unwatch`: Stop watching a specified file or directory.

## License
[![GNU AGPLv3 Image](https://www.gnu.org/graphics/agplv3-155x51.png)](https://www.gnu.org/licenses/agpl-3.0.html)  

This program is Free Software: You can use, study share and improve it at your
will. Specifically you can redistribute and/or modify it under the terms of the
[GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.html) as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
