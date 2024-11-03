package s3

import (
	"bytes"
	"context"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/astromechza/memory-mouse/internal/testsupport"
)

// sudo docker run --rm -it -p 4566:4566 --name localstack localstack/localstack
// sudo docker exec localstack awslocal s3api create-bucket --bucket smoke --region us-east-1
// S3_SMOKE_TEST_BUCKET_URL=http://localhost:4566/smoke
// S3_SMOKE_TEST_BUCKET_REGION=us-east-1
// S3_SMOKE_TEST_ACCESS_KEY_ID=na
// S3_SMOKE_TEST_SECRET_ACCESS_KEY=na
func TestSmoke(t *testing.T) {
	v := os.Getenv("S3_SMOKE_TEST_BUCKET_URL")
	if v == "" {
		t.Skip("S3_SMOKE_TEST_BUCKET_URL not set")
		return
	}
	s, err := New(
		http.DefaultClient, os.Getenv("S3_SMOKE_TEST_BUCKET_URL"), os.Getenv("S3_SMOKE_TEST_BUCKET_REGION"),
		os.Getenv("S3_SMOKE_TEST_ACCESS_KEY_ID"), os.Getenv("S3_SMOKE_TEST_SECRET_ACCESS_KEY"),
	)
	if err != nil {
		t.Fatal(err)
	}

	pId := strconv.Itoa(rand.Int())
	dId := strconv.Itoa(rand.Int())

	testsupport.MustAssertEqual(t, s.PutBlob(context.Background(), pId, dId, "0001", map[string]string{"x": "y"}, []byte("example")), nil)

	ids, err := s.ListProjectIds(context.Background())
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertContains(t, ids, pId)

	ids, err = s.ListDocumentIds(context.Background(), pId)
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{dId})

	blobs, err := s.ListBlobs(context.Background(), pId, dId)
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertEqual(t, len(blobs), 1)
	testsupport.AssertEqual(t, blobs[0].Id, "0001")
	testsupport.AssertEqual(t, blobs[0].Size, 7)

	blob, err := s.HeadBlob(context.Background(), pId, dId, "0001")
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertEqual(t, blob.Id, "0001")
	testsupport.AssertEqual(t, blob.Metadata["x"], "y")
	testsupport.AssertEqual(t, blob.Size, 7)

	buff := new(bytes.Buffer)
	blob, err = s.GetBlob(context.Background(), pId, dId, "0001", buff)
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertEqual(t, blob.Id, "0001")
	testsupport.AssertEqual(t, blob.Metadata["x"], "y")
	testsupport.AssertEqual(t, blob.Size, 7)
	testsupport.AssertEqual(t, buff.String(), "example")

	testsupport.MustAssertEqual(t, s.DeleteBlobs(context.Background(), pId, dId, []string{"0001"}), nil)

	ids, err = s.ListProjectIds(context.Background())
	testsupport.MustAssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{})
}
