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
	unwatched bool
	isDir     bool
	founds    map[string]*fileWatcherRecord
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
}

func NewFileWatcher(updateInterval time.Duration, created func(path string, isDir bool), removed func(path string, isDir bool), modified func(path string, isDir bool)) *FileWatcher {
	concurrentCache := concurrentcache.NewConcurrentCache[*fileWatcherCache](newFileWatcherCache(), updateInterval, func(locker concurrentcache.Locker, cache *fileWatcherCache) {
		locker.Lock()
		targetsCopy := make(map[string]*fileWatcherTarget, len(cache.targets))
		for targetPath, target := range cache.targets {
			targetsCopy[targetPath] = target
		}
		locker.Unlock()

		for targetPath, target := range targetsCopy {
			locker.RLock()
			unwatched := target.unwatched
			locker.RUnlock()
			info, err := os.Stat(targetPath)
			if err != nil && os.IsNotExist(err) || unwatched {
				locker.Lock()
				if unwatched {
					delete(cache.targets, targetPath)
				}
				_, ok := target.founds[targetPath]
				if ok {
					delete(target.founds, targetPath)
				}
				locker.Unlock()
				if ok {
					removed(targetPath, target.isDir)
				}
			}
			var (
				modTime int64 = math.MinInt64
				isDir         = true
			)
			if err == nil && !unwatched {
				modTime = info.ModTime().UnixNano()
				isDir = info.IsDir()
				locker.Lock()
				_, ok := cache.targets[targetPath]
				if ok {
					cache.targets[targetPath].isDir = isDir
				}
				locker.Unlock()
			}
			if isDir {
				locker.Lock()
				foundsCopy := make(map[string]*fileWatcherRecord, len(target.founds))
				for foundPath, found := range target.founds {
					foundsCopy[foundPath] = found
				}
				locker.Unlock()

				filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
					delete(foundsCopy, path)
					if err != nil && os.IsNotExist(err) || unwatched {
						locker.Lock()
						found, ok := target.founds[path]
						if ok {
							delete(target.founds, path)
						}
						locker.Unlock()
						if ok {
							removed(path, found.isDir)
						}
						return err
					}
					if err != nil {
						return err
					}
					var modTime, foundModTime int64 = info.ModTime().UnixNano(), math.MinInt64
					isDir := info.IsDir()
					locker.Lock()
					found, ok := target.founds[path]
					if ok {
						foundModTime = found.modTime
						*target.founds[path] = fileWatcherRecord{modTime: modTime, isDir: isDir}
					} else {
						target.founds[path] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
					}
					locker.Unlock()
					if modTime == foundModTime {
						return nil
					}
					if ok {
						modified(path, isDir)
					} else {
						created(path, isDir)
					}
					return nil
				})

				for foundPath, found := range foundsCopy {
					info, err := os.Stat(foundPath)
					if err != nil && os.IsNotExist(err) || unwatched {
						locker.Lock()
						_, ok := target.founds[foundPath]
						if ok {
							delete(target.founds, foundPath)
						}
						locker.Unlock()
						if ok {
							removed(foundPath, found.isDir)
						}
						continue
					}
					if err != nil {
						continue
					}
					var modTime, foundModTime int64 = info.ModTime().UnixNano(), math.MinInt64
					isDir := info.IsDir()
					locker.Lock()
					_, ok := target.founds[foundPath]
					if ok {
						foundModTime = found.modTime
						*target.founds[foundPath] = fileWatcherRecord{modTime: modTime, isDir: isDir}
					} else {
						target.founds[foundPath] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
					}
					locker.Unlock()
					if modTime == foundModTime {
						continue
					}
					if ok {
						modified(foundPath, isDir)
					} else {
						created(foundPath, isDir)
					}
				}
			} else {
				var foundModTime int64 = math.MinInt64
				locker.Lock()
				found, ok := target.founds[targetPath]
				if ok {
					foundModTime = found.modTime
					*target.founds[targetPath] = fileWatcherRecord{modTime: modTime, isDir: isDir}
				} else {
					target.founds[targetPath] = &fileWatcherRecord{modTime: modTime, isDir: isDir}
				}
				locker.Unlock()
				if modTime == foundModTime {
					continue
				}
				if ok {
					modified(targetPath, isDir)
				} else {
					created(targetPath, isDir)
				}
			}
		}
	})

	return &FileWatcher{
		mtx:             sync.Mutex{},
		concurrentCache: concurrentCache,
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
		cache.targets[absPath] = &fileWatcherTarget{unwatched: false, isDir: isDir, founds: make(map[string]*fileWatcherRecord)}
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

	f.concurrentCache.AccessWrite(func(cache *fileWatcherCache) {
		_, ok := cache.targets[absPath]
		if ok {
			cache.targets[absPath].unwatched = true
		}
	})

	return nil
}
