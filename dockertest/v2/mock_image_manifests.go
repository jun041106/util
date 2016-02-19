// Copyright 2015 Apcera Inc. All rights reserved.

package v2

const (
	// libraryNatsLatestManifest is the manifest for the library/nats image.
	// Omits history for brevity.
	libraryNatsLatestManifest = `{"schemaVersion":1,"name":"library/nats","tag":"latest","architecture":"amd64","fsLayers":[{"blobSum":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"},{"blobSum":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"},{"blobSum":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"},{"blobSum":"sha256:3f9c4e15d7dcbe265a38cca404cb0525dbf1124b34b15bed9f4df5cdfbf82370"},{"blobSum":"sha256:8b754456ca2582d79d592eea1d0e1df8704bce2dff4f0931cf00007fed78275c"},{"blobSum":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"}]}`

	// libraryFoobarLatestManifest layers correspond to "some content" and "some other content".
	libraryFoobarLatestManifest = `{"schemaVersion":1,"name":"library/foobar","tag":"latest","architecture":"amd64","fsLayers":[{"blobSum":"sha256:290f493c44f5d63d06b374d0a5abd292fae38b92cab2fae5efefe1b0e9347f56"},{"blobSum":"sha256:f73f16ede021d01efecf627b5e658be52293f167cfe06c6b8d0e591cb25b68c9"}]}`
)
