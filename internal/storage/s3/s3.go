package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/astromechza/memory-mouse/internal/storage"
)

type HttpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Storage struct {
	client             HttpDoer
	clock              func() time.Time
	bucketUrl          *url.URL
	region             string
	awsAccessKeyId     string
	awsSecretAccessKey string
}

type listBucketResult struct {
	IsTruncated           bool                     `xml:"IsTruncated"`
	NextContinuationToken string                   `xml:"NextContinuationToken"`
	Contents              []listBucketObject       `xml:"Contents"`
	CommonPrefixes        []listBucketCommonPrefix `xml:"CommonPrefixes"`
}

type listBucketCommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listBucketObject struct {
	Key  string `xml:"Key"`
	Size int64  `xml:"Size"`
}

// listObjectsV2 performs https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html.
func (s *Storage) listObjectsV2(ctx context.Context, prefix, delimiter, continuationToken string) (*listBucketResult, error) {
	q := make(url.Values)
	q.Set("list-type", "2")
	if prefix != "" {
		q.Set("prefix", prefix)
	}
	if delimiter != "" {
		q.Set("delimiter", delimiter)
	}
	if continuationToken != "" {
		q.Set("continuation-token", continuationToken)
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, s.bucketUrl.ResolveReference(&url.URL{
		RawQuery: q.Encode(),
	}).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build list objects request: %w", err)
	}
	if err := signSigV4(r, s.clock, s.region, s.awsAccessKeyId, s.awsSecretAccessKey); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to make list objects request: %w", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to list objects: %s", resp.Status)
		}
		var out listBucketResult
		if err := xml.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("failed to decode list objects response: %w", err)
		}
		return &out, nil
	}
}

// listObjectsV2All performs https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html with pagination.
func (s *Storage) listObjectsV2All(ctx context.Context, prefix, delimiter string) (*listBucketResult, error) {
	out := &listBucketResult{}
	continuationToken := ""
	for {
		r, err := s.listObjectsV2(ctx, prefix, delimiter, continuationToken)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		out.Contents = append(out.Contents, r.Contents...)
		out.CommonPrefixes = append(out.CommonPrefixes, r.CommonPrefixes...)
		if r.IsTruncated {
			continuationToken = r.NextContinuationToken
		} else {
			break
		}
	}
	return out, nil
}

func (s *Storage) ListProjectIds(ctx context.Context) (projectIds []string, err error) {
	r, err := s.listObjectsV2All(ctx, "", "/")
	if err != nil {
		return nil, fmt.Errorf("failed to list all objects: %w", err)
	}
	projectIds = make([]string, 0, len(r.CommonPrefixes))
	for _, prefix := range r.CommonPrefixes {
		projectIds = append(projectIds, strings.TrimSuffix(prefix.Prefix, "/"))
	}
	return projectIds, nil
}

func (s *Storage) ListDocumentIds(ctx context.Context, projectId string) (documentIds []string, err error) {
	prefix := projectId + "/"
	r, err := s.listObjectsV2All(ctx, prefix, "/")
	if err != nil {
		return nil, fmt.Errorf("failed to list all objects: %w", err)
	}
	documentIds = make([]string, 0, len(r.CommonPrefixes))
	for _, p := range r.CommonPrefixes {
		documentIds = append(documentIds, strings.TrimSuffix(strings.TrimPrefix(p.Prefix, prefix), "/"))
	}
	return documentIds, nil
}

func (s *Storage) ListBlobs(ctx context.Context, projectId, documentId string) (blobs []storage.BlobIdAndSize, err error) {
	prefix := fmt.Sprintf("%s/%s/", projectId, documentId)
	r, err := s.listObjectsV2All(ctx, prefix, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list all objects: %w", err)
	}
	blobs = make([]storage.BlobIdAndSize, 0, len(r.Contents))
	for _, content := range r.Contents {
		blobs = append(blobs, storage.BlobIdAndSize{
			Id:   strings.TrimPrefix(content.Key, prefix),
			Size: content.Size,
		})
	}
	return blobs, nil
}

func (s *Storage) PutBlob(ctx context.Context, projectId, documentId, blobId string, meta map[string]string, blob []byte) error {
	key := fmt.Sprintf("%s/%s/%s", projectId, documentId, blobId)
	r, err := http.NewRequestWithContext(ctx, http.MethodPut, s.bucketUrl.ResolveReference(&url.URL{Path: key}).String(), bytes.NewReader(blob))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	for s2, s3 := range meta {
		r.Header.Set("x-amz-meta-"+s2, s3)
	}
	if err := signSigV4(r, s.clock, s.region, s.awsAccessKeyId, s.awsSecretAccessKey); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to make request due to status code: %s", resp.Status)
		}
	}
	return nil
}

func (s *Storage) readBlob(ctx context.Context, projectId, documentId, blobId, method string, dst io.Writer) (blob *storage.BlobIdSizeAndMeta, err error) {
	key := fmt.Sprintf("%s/%s/%s", projectId, documentId, blobId)
	r, err := http.NewRequestWithContext(ctx, method, s.bucketUrl.ResolveReference(&url.URL{Path: key}).String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	if err := signSigV4(r, s.clock, s.region, s.awsAccessKeyId, s.awsSecretAccessKey); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to make request due to status code: %s", resp.Status)
		}
		if dst != nil {
			if _, err := io.Copy(dst, resp.Body); err != nil {
				return nil, fmt.Errorf("failed to copy response body: %w", err)
			}
		}
	}
	outMeta := make(map[string]string)
	for k, v := range resp.Header {
		k = strings.ToLower(k)
		if strings.HasPrefix(k, "x-amz-meta-") {
			outMeta[strings.TrimPrefix(k, "x-amz-meta-")] = v[0]
		}
	}
	return &storage.BlobIdSizeAndMeta{
		BlobIdAndSize: storage.BlobIdAndSize{
			Id:   blobId,
			Size: resp.ContentLength,
		},
		Metadata: outMeta,
	}, nil
}

func (s *Storage) GetBlob(ctx context.Context, projectId, documentId, blobId string, dst io.Writer) (blob *storage.BlobIdSizeAndMeta, err error) {
	return s.readBlob(ctx, projectId, documentId, blobId, http.MethodGet, dst)
}

func (s *Storage) HeadBlob(ctx context.Context, projectId, documentId, blobId string) (blob *storage.BlobIdSizeAndMeta, err error) {
	return s.readBlob(ctx, projectId, documentId, blobId, http.MethodHead, io.Discard)
}

type deleteObjectsBody struct {
	XMLName xml.Name              `xml:"Delete"`
	Objects []deleteObjectsObject `xml:"Object"`
}

type deleteObjectsObject struct {
	Key string `xml:"Key"`
}

func (s *Storage) DeleteBlobs(ctx context.Context, projectId, documentId string, blobIds []string) error {
	body := &deleteObjectsBody{Objects: make([]deleteObjectsObject, 0, len(blobIds))}
	for _, id := range blobIds {
		body.Objects = append(body.Objects, deleteObjectsObject{Key: fmt.Sprintf("%s/%s/%s", projectId, documentId, id)})
	}
	rawBod, _ := xml.Marshal(body)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, s.bucketUrl.ResolveReference(&url.URL{RawQuery: "delete"}).String(), bytes.NewReader(rawBod))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if err := signSigV4(r, s.clock, s.region, s.awsAccessKeyId, s.awsSecretAccessKey); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to make request due to status code: %s", resp.Status)
		}
	}
	return nil
}

func New(client HttpDoer, bucketUrl string, region, awsAccessKeyId, awsSecretAccessKey string) (*Storage, error) {
	u, err := url.Parse(bucketUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse S3 URL: %w", err)
	} else if u.Path != "" && !strings.HasSuffix(u.Path, "/") {
		return nil, fmt.Errorf("bucket url path must end with /")
	}
	return &Storage{
		client:             client,
		clock:              time.Now,
		bucketUrl:          u,
		region:             region,
		awsAccessKeyId:     awsAccessKeyId,
		awsSecretAccessKey: awsSecretAccessKey,
	}, nil
}

var _ storage.BlobStorage = (*Storage)(nil)
