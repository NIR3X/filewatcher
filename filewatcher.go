package filewatcher

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NIR3X/concurrentcache"
)

type fileWatcherRecord struct {
	modTime int64
	isDir   bool
}

type fileWatcherTarget struct {
	isDir  bool
	founds map[string]*fileWatcherRecord
}

type fileWatcherCache struct {
	targets map[string]*fileWatcherTarget
}

func newFileWatcherCache() *fileWatcherCache {
	return &fileWatcherCache{
		targets: make(map[string]*fileWatcherTarget),
	}
}

type FileWatcher struct {
	concurrentCache concurrentcache.ConcurrentCache[*fileWatcherCache]
	created         func(path string, isDir bool)
	removed         func(path string, isDir bool)
	modified        func(path string, isDir bool)
}

func handleFound(target *fileWatcherTarget, path string, info os.FileInfo, err error, created func(path string, isDir bool), removed func(path string, isDir bool), modified func(path string, isDir bool)) {
	var foundIsDir bool
	found, ok := target.founds[path]
	if ok {
		foundIsDir = found.isDir
	}
	if err != nil {
		if os.IsNotExist(err) {
			if ok {
				delete(target.founds, path)
				removed(path, foundIsDir)
			}
		}
		return
	}
	isDir := info.IsDir()
	modTime := info.ModTime().UnixNano()
	if ok {
		isModified := modTime != found.modTime
		switch {
		case isDir != foundIsDir:
			defer func() {
				removed(path, foundIsDir)
				created(path, isDir)
			}()
		case isModified:
			defer modified(path, isDir)
		default:
			return
		}
		*target.founds[path] = fileWatcherRecord{modTime: modTime, isDir: isDir}
	} else {
		target.founds[path] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
		created(path, isDir)
	}
}

func NewFileWatcher(updateInterval time.Duration, created func(path string, isDir bool), removed func(path string, isDir bool), modified func(path string, isDir bool)) *FileWatcher {
	concurrentCache := concurrentcache.NewConcurrentCache[*fileWatcherCache](newFileWatcherCache(), updateInterval, func(locker concurrentcache.Locker, cache *fileWatcherCache) {
		locker.Lock()
		defer locker.Unlock()

		for targetPath, target := range cache.targets {
			foundsPath := make(filePathList, 0, len(target.founds))
			for foundPath := range target.founds {
				foundsPath = append(foundsPath, filePath{Value: foundPath, Depth: strings.Count(foundPath, string(os.PathSeparator))})
			}

			if target.isDir {
				filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
					if _, ok := target.founds[path]; !ok {
						foundsPath = append(foundsPath, filePath{Value: path, Depth: strings.Count(path, string(os.PathSeparator))})
					}
					return nil
				})
			}

			sort.Sort(foundsPath)

			for _, foundPath := range foundsPath {
				info, err := os.Stat(foundPath.Value)
				handleFound(target, foundPath.Value, info, err, created, removed, modified)
			}
		}
	})

	return &FileWatcher{
		concurrentCache: concurrentCache,
		created:         created,
		removed:         removed,
		modified:        modified,
	}
}

func (f *FileWatcher) Close() {
	f.concurrentCache.Close()
}

func (f *FileWatcher) Watch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	isDir := info.IsDir()

	f.concurrentCache.AccessWrite(func(cache *fileWatcherCache) {
		if _, ok := cache.targets[absPath]; !ok {
			cache.targets[absPath] = &fileWatcherTarget{isDir: isDir, founds: make(map[string]*fileWatcherRecord)}
		}
	})

	return nil
}

func (f *FileWatcher) Unwatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	f.concurrentCache.AccessWrite(func(cache *fileWatcherCache) {
		target, ok := cache.targets[absPath]
		if ok {
			delete(cache.targets, absPath)
		} else {
			return
		}

		foundsPath := make(filePathList, 0, len(target.founds))
		for foundPath := range target.founds {
			foundsPath = append(foundsPath, filePath{Value: foundPath, Depth: strings.Count(foundPath, string(os.PathSeparator))})
		}

		sort.Sort(foundsPath)

		for _, foundPath := range foundsPath {
			found, _ := target.founds[foundPath.Value]
			delete(target.founds, foundPath.Value)
			f.removed(foundPath.Value, found.isDir)
		}
	})

	return nil
}
