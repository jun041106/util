package memory

import (
	"testing"

	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/cache/cachecheck"
)

// TestInMemoryBlobInfoCache checks the in memory implementation is working
// correctly.
func TestInMemoryBlobInfoCache(t *testing.T) {
	cachecheck.CheckBlobDescriptorCache(t, NewInMemoryBlobDescriptorCacheProvider())
}
