package main

import (
	_ "net/http/pprof"

	"github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/auth/htpasswd"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/auth/silly"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/auth/token"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/proxy"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/azure"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/gcs"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/inmemory"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/middleware/cloudfront"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/oss"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/s3"
	_ "github.com/apcera/util/docker/v2/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/swift"
)

func main() {
	registry.Cmd.Execute()
}
