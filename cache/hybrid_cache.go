package cache

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

const (
	defaultMaxMemoryItems = 1000
	defaultFlushInterval  = 5 * time.Minute
)

// HybridCache combines in-memory LRU cache with disk persistence
type HybridCache struct {
	mu          sync.RWMutex
	memCache    map[string]*list.Element
	lru         *list.List
	maxItems    int
	diskCache   *FileCache
	flushTicker *time.Ticker
	stopFlush   chan struct{}
	dirtyKeys   map[string]struct{} // Track modified items for flush
}

// cacheItem represents an item in the LRU cache
type cacheItem struct {
	key        string
	entry      Entry // Use Entry from file_cache.go
	lastAccess time.Time
}

// NewHybridCache creates a new hybrid cache with LRU eviction
func NewHybridCache(baseDir string, maxItems int) (*HybridCache, error) {
	if maxItems <= 0 {
		maxItems = defaultMaxMemoryItems
	}

	diskCache, err := New(baseDir)
	if err != nil {
		return nil, err
	}

	hc := &HybridCache{
		memCache:    make(map[string]*list.Element, maxItems),
		lru:         list.New(),
		maxItems:    maxItems,
		diskCache:   diskCache,
		flushTicker: time.NewTicker(defaultFlushInterval),
		stopFlush:   make(chan struct{}),
		dirtyKeys:   make(map[string]struct{}, maxItems/10),
	}

	// Start background flush goroutine
	// Capture cache reference to avoid race condition warnings
	cache := hc
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Log the panic and continue
				fmt.Printf("Hybrid cache background flush goroutine recovered from panic: %v\n", recovered)
			}
		}()
		cache.backgroundFlush()
	}()

	// Load hot data from disk cache into memory
	hc.preloadHotData()

	return hc, nil
}

// Get retrieves an item from cache (memory first, then disk)
func (hc *HybridCache) Get(key string) (*Entry, bool) {
	hc.mu.RLock()
	if elem, ok := hc.memCache[key]; ok {
		hc.mu.RUnlock()
		// Move to front (most recently used)
		hc.mu.Lock()
		hc.lru.MoveToFront(elem)
		item, _ := elem.Value.(*cacheItem)
		item.lastAccess = time.Now()
		hc.mu.Unlock()
		return &item.entry, true
	}
	hc.mu.RUnlock()

	// Check disk cache
	if record, err := hc.diskCache.GetFileRecord(key); err == nil {
		entry := Entry{
			Hash:   record.Hash,
			Issues: record.Issues, // Already []*Issue type
		}

		// Add to memory cache
		hc.mu.Lock()
		hc.addToMemCache(key, entry)
		hc.mu.Unlock()

		return &entry, true
	}

	return nil, false
}

// Put adds or updates an item in the cache
func (hc *HybridCache) Put(key string, entry Entry) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Mark as dirty for flush
	hc.dirtyKeys[key] = struct{}{}

	// Check if key exists
	if elem, ok := hc.memCache[key]; ok {
		// Update existing
		hc.lru.MoveToFront(elem)
		item, _ := elem.Value.(*cacheItem)
		item.entry = entry
		item.lastAccess = time.Now()
		return
	}

	// Add new item
	hc.addToMemCache(key, entry)
}

// addToMemCache adds item to memory cache with LRU eviction
func (hc *HybridCache) addToMemCache(key string, entry Entry) {
	// Check if we need to evict
	if hc.lru.Len() >= hc.maxItems {
		hc.evictOldest()
	}

	item := &cacheItem{
		key:        key,
		entry:      entry,
		lastAccess: time.Now(),
	}
	elem := hc.lru.PushFront(item)
	hc.memCache[key] = elem
}

// evictOldest removes the least recently used item
func (hc *HybridCache) evictOldest() {
	elem := hc.lru.Back()
	if elem != nil {
		hc.lru.Remove(elem)
		item, _ := elem.Value.(*cacheItem)
		delete(hc.memCache, item.key)

		// If dirty, flush to disk before eviction
		if _, isDirty := hc.dirtyKeys[item.key]; isDirty {
			hc.flushItem(item.key, item.entry)
			delete(hc.dirtyKeys, item.key)
		}
	}
}

// flushItem writes a single item to disk
func (hc *HybridCache) flushItem(key string, entry Entry) {
	// Entry is already the right type, just pass it directly
	_ = hc.diskCache.SaveRawRecord(key, entry)
}

// Flush writes all dirty items to disk
func (hc *HybridCache) Flush() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for key := range hc.dirtyKeys {
		if elem, ok := hc.memCache[key]; ok {
			item, _ := elem.Value.(*cacheItem)
			hc.flushItem(key, item.entry)
		}
	}

	// Clear dirty keys
	hc.dirtyKeys = make(map[string]struct{}, len(hc.dirtyKeys))

	return hc.diskCache.save()
}

// backgroundFlush periodically flushes dirty items to disk
func (hc *HybridCache) backgroundFlush() {
	for {
		select {
		case <-hc.flushTicker.C:
			_ = hc.Flush()
		case <-hc.stopFlush:
			return
		}
	}
}

// preloadHotData loads frequently accessed data into memory
func (hc *HybridCache) preloadHotData() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	// Load up to maxItems/2 most recent items
	count := 0
	maxPreload := hc.maxItems / 2

	for _, record := range hc.diskCache.data.Files {
		if count >= maxPreload {
			break
		}

		// Only preload recent items (last 24 hours)
		if time.Since(record.LastAnalyzed) < 24*time.Hour {
			entry := Entry{
				Hash:   record.Hash,
				Issues: record.Issues, // Already []*Issue type
			}
			hc.addToMemCache(record.Path, entry)
			count++
		}
	}
}

// Close flushes and closes the cache
func (hc *HybridCache) Close() error {
	close(hc.stopFlush)
	hc.flushTicker.Stop()

	// Final flush
	if err := hc.Flush(); err != nil {
		return err
	}

	return hc.diskCache.Close()
}

// Stats return cache statistics
func (hc *HybridCache) Stats() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	diskStats := hc.diskCache.GetStats()

	return map[string]interface{}{
		"memory_items":      hc.lru.Len(),
		"memory_capacity":   hc.maxItems,
		"dirty_items":       len(hc.dirtyKeys),
		"disk_total_files":  diskStats["total_files"],
		"disk_cache_hits":   diskStats["cache_hits"],
		"disk_cache_misses": diskStats["cache_misses"],
	}
}

// Clear clears both memory and disk cache
func (hc *HybridCache) Clear() error {
	hc.mu.Lock()
	hc.memCache = make(map[string]*list.Element, hc.maxItems)
	hc.lru = list.New()
	hc.dirtyKeys = make(map[string]struct{}, hc.maxItems/10)
	hc.mu.Unlock()

	return hc.diskCache.ClearCache()
}
