package filewatcher

import (
	"math"
	"os"
	"path/filepath"
	"sync"
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
	mtx             sync.Mutex
	concurrentCache concurrentcache.ConcurrentCache[*fileWatcherCache]
	created         func(path string, isDir bool)
	removed         func(path string, isDir bool)
	modified        func(path string, isDir bool)
}

func handleFound(locker concurrentcache.Locker, target *fileWatcherTarget, path string, info os.FileInfo, err error, created func(path string, isDir bool), removed func(path string, isDir bool), modified func(path string, isDir bool)) {
	var (
		foundIsDir   bool
		foundModTime int64 = math.MinInt64
	)
	locker.RLock()
	found, ok := target.founds[path]
	if ok {
		foundIsDir = found.isDir
		foundModTime = found.modTime
	}
	locker.RUnlock()

	if err != nil {
		if os.IsNotExist(err) {
			if ok {
				locker.Lock()
				delete(target.founds, path)
				locker.Unlock()
				removed(path, foundIsDir)
			}
		}
		return
	}
	isDir := info.IsDir()
	modTime := info.ModTime().UnixNano()
	if ok {
		isModified := modTime != foundModTime
		if isDir != foundIsDir {
			defer func() {
				removed(path, foundIsDir)
				created(path, isDir)
			}()
		} else if isModified {
			defer func() {
				modified(path, isDir)
			}()
		} else {
			return
		}
		locker.Lock()
		if _, ok := target.founds[path]; ok {
			*target.founds[path] = fileWatcherRecord{modTime: modTime, isDir: isDir}
		} else {
			target.founds[path] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
		}
		locker.Unlock()
	} else {
		locker.Lock()
		target.founds[path] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
		locker.Unlock()
		created(path, isDir)
	}
}

func NewFileWatcher(updateInterval time.Duration, created func(path string, isDir bool), removed func(path string, isDir bool), modified func(path string, isDir bool)) *FileWatcher {
	concurrentCache := concurrentcache.NewConcurrentCache[*fileWatcherCache](newFileWatcherCache(), updateInterval, func(locker concurrentcache.Locker, cache *fileWatcherCache) {
		locker.RLock()
		targetsCopy := mapDup(cache.targets)
		locker.RUnlock()

		for targetPath, target := range targetsCopy {
			locker.RLock()
			isDir := target.isDir
			foundsCopy := mapDup(target.founds)
			locker.RUnlock()

			if isDir {
				filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
					if _, ok := foundsCopy[path]; ok {
						delete(foundsCopy, path)
					}
					handleFound(locker, target, path, info, err, created, removed, modified)
					return nil
				})
			}

			for foundPath := range foundsCopy {
				info, err := os.Stat(foundPath)
				handleFound(locker, target, foundPath, info, err, created, removed, modified)
			}
		}
	})

	return &FileWatcher{
		mtx:             sync.Mutex{},
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
	f.mtx.Lock()
	defer f.mtx.Unlock()

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
		cache.targets[absPath] = &fileWatcherTarget{isDir: isDir, founds: make(map[string]*fileWatcherRecord)}
	})

	return nil
}

func (f *FileWatcher) Unwatch(path string) error {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	f.concurrentCache.Access(func(locker concurrentcache.Locker, cache *fileWatcherCache) {
		var foundsClone map[string]*fileWatcherRecord
		locker.RLock()
		target, ok := cache.targets[absPath]
		if ok {
			foundsClone = mapDup(target.founds)
		}
		locker.RUnlock()
		if ok {
			locker.Lock()
			delete(cache.targets, absPath)
			locker.Unlock()
		} else {
			return
		}

		for foundPath, found := range foundsClone {
			locker.Lock()
			isDir := found.isDir
			delete(target.founds, foundPath)
			locker.Unlock()
			f.removed(foundPath, isDir)
		}
	})

	return nil
}
