package inmemory

import (
	"testing"

	storagedriver "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver"
	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/testsuites"
	"gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

func init() {
	inmemoryDriverConstructor := func() (storagedriver.StorageDriver, error) {
		return New(), nil
	}
	testsuites.RegisterSuite(inmemoryDriverConstructor, testsuites.NeverSkip)
}
