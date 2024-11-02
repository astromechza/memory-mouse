package storage

import (
	"context"
	"errors"
	"io"
)

var ErrDocumentNotFound = errors.New("document not found")
var ErrBlobNotFound = errors.New("blob not found")

// BlobStorage is our abstraction over the backing storage interface whether it is an object storage api or another
// backing storage like Sqlite or DuckDB.
type BlobStorage interface {
	// ListProjectIds is generally internal only for us to find all the projects and fully enumerate the space.
	ListProjectIds(ctx context.Context) (projectIds []string, err error)
	// ListDocumentIds allows us to list the document ids under a project. One day we might need to be able to retrieve
	// more information per document, like descriptions or estimated sizes and things, but for now this api just returns
	// the basic ids - which should always be possible without much stress.
	ListDocumentIds(ctx context.Context, projectId string) (documentIds []string, err error)
	// ListBlobs should list all the blobs for a document. No particular order is assumed, so the ids themselves should
	// help to indicate the desired order. May return ErrDocumentNotFound.
	ListBlobs(ctx context.Context, projectId, documentId string) (blobIds []string, err error)
	// PutBlob will write or overwrite the target blob and set the given metadata. The blob is specified as a byte array
	// rather than an io reader because the blobs are assumed to be in memory document dumps and we need a good way
	// to read the document twice to calculate checksums and apply encryption, etc. This results in the creation of
	// a document if it doesn't already exist, so this API will NOT return ErrBlobNotFound or ErrDocumentNotFound.
	PutBlob(ctx context.Context, projectId, documentId, blobId string, meta map[string]string, blob []byte) error
	// GetBlob retrieves the blob contents as a writer. This may return ErrBlobNotFound or ErrDocumentNotFound.
	GetBlob(ctx context.Context, projectId, documentId, blobId string, dst io.Writer) (n int, meta map[string]string, err error)
	// HeadBlob is the same as GetBlob but doesn't retrieve the content. This may return ErrBlobNotFound or ErrDocumentNotFound.
	HeadBlob(ctx context.Context, projectId, documentId, blobId string) (n int, meta map[string]string, err error)
	// DeleteBlobs deletes one or more blobs by id from the storage. This may return ErrDocumentNotFound but does not
	// return ErrBlobNotFound - missing blobs are treated as deleted.
	DeleteBlobs(ctx context.Context, projectId, documentId string, blobIds []string) error
}
