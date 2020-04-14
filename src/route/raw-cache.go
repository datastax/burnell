package route

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	// "github.com/apex/log"
	"github.com/allegro/bigcache"
)

var HttpCache *bigcache.BigCache

func InitCache() {
	var err error
	HttpCache, err = bigcache.NewBigCache(bigcache.Config{
		// number of shards (must be a power of 2)
		Shards: 128,

		// time after which entry can be evicted
		LifeWindow: 20 * time.Second,

		// Interval between removing expired entries (clean up).
		// If set to <= 0 then no action is performed.
		// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
		CleanWindow: 5 * time.Minute,

		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: 1000 * 10 * 60,

		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: 500,

		// prints information about additional memory allocation
		Verbose: true,

		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 64,

		// callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: nil,

		// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A constant representing the reason will be passed through.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		// Ignored if OnRemove is specified.
		OnRemoveWithReason: nil,
	})
	if err != nil {
		log.Fatalln("failed to initialize bigcache", err)
	}
}

func HashKey(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
