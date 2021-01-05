// +build azblob_oldapi
//
// This file provides a wrapper around azure/azblob PageBlobURL.
//
// Version 0.12 of the azblob library changed the API of PageBlobURL.
// (see https://github.com/Azure/azure-storage-blob-go/blob/master/BreakingChanges.md)
// This means that different APIs are available in Fedora 32 and 33 (it does
// not matter for RHEL as it uses vendored libraries).
// This wrapper allows us to use both azblob's APIs using buildflags.
//
// This file is a wrapper for azblob older than 0.12.

package azure

import (
	"context"
	"io"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

type PageBlobURL struct {
	impl azblob.PageBlobURL
}

func newPageBlobURL(containerURL azblob.ContainerURL, blobName string) PageBlobURL {
	pageblobURL := containerURL.NewPageBlobURL(blobName)

	return PageBlobURL{pageblobURL}
}

func (pb PageBlobURL) Create(ctx context.Context, size int64, sequenceNumber int64, h azblob.BlobHTTPHeaders, metadata azblob.Metadata, ac azblob.BlobAccessConditions) (*azblob.PageBlobCreateResponse, error) {
	return pb.impl.Create(ctx, size, sequenceNumber, h, metadata, ac)
}

func (pb PageBlobURL) SetHTTPHeaders(ctx context.Context, h azblob.BlobHTTPHeaders, ac azblob.BlobAccessConditions) (*azblob.BlobSetHTTPHeadersResponse, error) {
	return pb.impl.SetHTTPHeaders(ctx, h, ac)
}

func (pb PageBlobURL) UploadPages(ctx context.Context, offset int64, body io.ReadSeeker, ac azblob.PageBlobAccessConditions, transactionalMD5 []byte) (*azblob.PageBlobUploadPagesResponse, error) {
	return pb.impl.UploadPages(ctx, offset, body, ac, transactionalMD5)
}

func (pb PageBlobURL) GetProperties(ctx context.Context, ac azblob.BlobAccessConditions) (*azblob.BlobGetPropertiesResponse, error) {
	return pb.impl.GetProperties(ctx, ac)
}
