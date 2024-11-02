package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/astromechza/memory-mouse/internal/testsupport"
)

func randomInMemoryDbString() string {
	return fmt.Sprintf("file:db-%d.db?cache=shared&mode=memory", rand.Int64())
}

// Test just runs the set of nominal operations
func Test(t *testing.T) {
	t.Parallel()
	s, err := New(context.Background(), randomInMemoryDbString(), 2)
	testsupport.AssertEqual(t, err, nil)

	pId := strconv.Itoa(rand.Int())
	dId := strconv.Itoa(rand.Int())

	testsupport.AssertEqual(t, s.PutBlob(context.Background(), pId, dId, "0001", map[string]string{"x": "y"}, []byte("example")), nil)

	ids, err := s.ListProjectIds(context.Background())
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{pId})

	ids, err = s.ListDocumentIds(context.Background(), pId)
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{dId})

	ids, err = s.ListBlobs(context.Background(), pId, dId)
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{"0001"})

	n, meta, err := s.HeadBlob(context.Background(), pId, dId, "0001")
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, meta["x"], "y")
	testsupport.AssertEqual(t, n, 7)

	buff := new(bytes.Buffer)
	n, meta, err = s.GetBlob(context.Background(), pId, dId, "0001", buff)
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, meta["x"], "y")
	testsupport.AssertEqual(t, n, 7)
	testsupport.AssertEqual(t, buff.String(), "example")

	testsupport.AssertEqual(t, s.DeleteBlobs(context.Background(), pId, dId, []string{"0001"}), nil)

	ids, err = s.ListProjectIds(context.Background())
	testsupport.AssertEqual(t, err, nil)
	testsupport.AssertEqual(t, ids, []string{})
}
