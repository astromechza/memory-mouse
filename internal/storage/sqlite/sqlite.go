package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/astromechza/memory-mouse/internal/storage"
)

type Storage struct {
	writer *sql.DB
	reader *sql.DB
}

func newConn(connString string, maxConnections int) (*sql.DB, error) {
	slog.Debug("opening connection", slog.String("conn", connString))
	db, err := sql.Open("sqlite3", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open sql: %w", err)
	}
	db.SetMaxOpenConns(maxConnections)
	db.SetMaxIdleConns(maxConnections)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	return db, nil
}

func New(ctx context.Context, connString string, dedicatedReaders int) (*Storage, error) {
	// In theory, we could open the same reader twice, once for writers and once for readers. But right now that's not
	// really necessary.

	writer, err := newConn(connString, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to open sql: %w", err)
	}
	var good bool
	defer func() {
		if !good {
			slog.Info("closing reader connection due to failed setup")
			if err := writer.Close(); err != nil {
				slog.Warn("failed to close reader connection due to failed setup", slog.Any("err", err))
			}
		}
	}()

	slog.Debug("beginning table assert transaction")
	if _, err := writer.ExecContext(
		ctx, `CREATE TABLE IF NOT EXISTS blobs (
    project_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    blob_id TEXT NOT NULL,
    meta_json TEXT NOT NULL,
    content BLOB NOT NULL,
    PRIMARY KEY(project_id, document_id, blob_id)
)`,
	); err != nil {
		return nil, fmt.Errorf("failed to execute statement: %w", err)
	}

	reader := writer
	if dedicatedReaders > 0 {
		reader, err = newConn(connString, dedicatedReaders)
		if err != nil {
			return nil, fmt.Errorf("failed to open dedicated reader db")
		}
	}

	good = true
	return &Storage{writer: writer, reader: reader}, nil
}

func (s *Storage) Close() error {
	return errors.Join(s.writer.Close(), s.reader.Close())
}

func (s *Storage) ListProjectIds(ctx context.Context) (projectIds []string, err error) {
	slog.Debug("executing list project ids query")
	if r, err := s.reader.QueryContext(ctx, `SELECT DISTINCT project_id FROM blobs`); err != nil {
		return nil, fmt.Errorf("failed to perform list project ids query: %w", err)
	} else {
		defer func() {
			if err := r.Close(); err != nil {
				slog.Warn("failed to close query", slog.Any("err", err))
			}
		}()
		out := make([]string, 0)
		for r.Next() {
			var id string
			if err := r.Scan(&id); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}
			out = append(out, id)
		}
		if err := r.Err(); err != nil {
			return nil, fmt.Errorf("failed to iterate rows: %w", err)
		}
		return out, nil
	}
}

func (s *Storage) ListDocumentIds(ctx context.Context, projectId string) (documentIds []string, err error) {
	slog.Debug("executing list document ids query", slog.String("project", projectId))
	if r, err := s.reader.QueryContext(ctx, `SELECT DISTINCT document_id FROM blobs WHERE project_id = $1`, projectId); err != nil {
		return nil, fmt.Errorf("failed to perform list document ids query: %w", err)
	} else {
		defer func() {
			if err := r.Close(); err != nil {
				slog.Warn("failed to close query", slog.Any("err", err))
			}
		}()
		out := make([]string, 0)
		for r.Next() {
			var id string
			if err := r.Scan(&id); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}
			out = append(out, id)
		}
		if err := r.Err(); err != nil {
			return nil, fmt.Errorf("failed to iterate rows: %w", err)
		}
		return out, nil
	}
}

func (s *Storage) ListBlobs(ctx context.Context, projectId, documentId string) (blobs []storage.BlobIdAndSize, err error) {
	slog.Debug("executing list blob ids query", slog.String("project", projectId), slog.String("document", documentId))
	if r, err := s.reader.QueryContext(ctx, `SELECT blob_id, length(content) FROM blobs WHERE project_id = $1 AND document_id = $2`, projectId, documentId); err != nil {
		return nil, fmt.Errorf("failed to perform list blob ids query: %w", err)
	} else {
		defer func() {
			if err := r.Close(); err != nil {
				slog.Warn("failed to close query", slog.Any("err", err))
			}
		}()
		out := make([]storage.BlobIdAndSize, 0)
		for r.Next() {
			var id string
			var size int64
			if err := r.Scan(&id, &size); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}
			out = append(out, storage.BlobIdAndSize{Id: id, Size: size})
		}
		if err := r.Err(); err != nil {
			return nil, fmt.Errorf("failed to iterate rows: %w", err)
		}
		return out, nil
	}
}

func (s *Storage) PutBlob(ctx context.Context, projectId, documentId, blobId string, meta map[string]string, blob []byte) error {
	if meta == nil {
		meta = map[string]string{}
	}
	metaRaw, _ := json.Marshal(meta)
	slog.Debug("executing put blob", slog.String("project", projectId), slog.String("document", documentId), slog.String("blob", blobId), slog.Int("#content", len(blob)))
	if r, err := s.writer.ExecContext(ctx, `INSERT INTO blobs VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO UPDATE SET meta_json = $4, content = $5`, projectId, documentId, blobId, metaRaw, blob); err != nil {
		return fmt.Errorf("failed to perform put blob query: %w", err)
	} else if rc, _ := r.RowsAffected(); rc != 1 {
		return fmt.Errorf("failed to perform put blob query: expected 1 row affected, got %d", rc)
	}
	return nil
}

func (s *Storage) GetBlob(ctx context.Context, projectId, documentId, blobId string, dst io.Writer) (blob *storage.BlobIdSizeAndMeta, err error) {
	slog.Debug("executing get blob", slog.String("project", projectId), slog.String("document", documentId), slog.String("blob", blobId))
	var metaRaw string
	var content []byte
	if err := s.reader.QueryRowContext(
		ctx, `SELECT meta_json, content FROM blobs WHERE project_id = $1 AND document_id = $2 AND blob_id = $3`,
		projectId, documentId, blobId,
	).Scan(&metaRaw, &content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrBlobNotFound
		}
		return nil, fmt.Errorf("failed to perform get blob query: %w", err)
	}
	out := &storage.BlobIdSizeAndMeta{BlobIdAndSize: storage.BlobIdAndSize{Id: blobId, Size: int64(len(content))}}
	if err := json.Unmarshal([]byte(metaRaw), &out.Metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata from blob query: %w", err)
	} else if _, err := dst.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write blob: %w", err)
	} else {
		return out, nil
	}
}

func (s *Storage) HeadBlob(ctx context.Context, projectId, documentId, blobId string) (blob *storage.BlobIdSizeAndMeta, err error) {
	slog.Debug("executing head blob", slog.String("project", projectId), slog.String("document", documentId), slog.String("blob", blobId))
	var metaRaw string
	out := &storage.BlobIdSizeAndMeta{BlobIdAndSize: storage.BlobIdAndSize{Id: blobId}}
	if err := s.reader.QueryRowContext(
		ctx, `SELECT meta_json, length(content) FROM blobs WHERE project_id = $1 AND document_id = $2 AND blob_id = $3`,
		projectId, documentId, blobId,
	).Scan(&metaRaw, &out.Size); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrBlobNotFound
		}
		return nil, fmt.Errorf("failed to perform get blob query: %w", err)
	}
	if err := json.Unmarshal([]byte(metaRaw), &out.Metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata from blob query: %w", err)
	}
	return out, nil
}

func (s *Storage) DeleteBlobs(ctx context.Context, projectId, documentId string, blobIds []string) error {
	slog.Debug("executing delete blobs", slog.String("project", projectId), slog.String("document", documentId), slog.Int("#blobs", len(blobIds)))
	if len(blobIds) == 0 {
		return nil
	}
	args := append(make([]interface{}, 0, 2+len(blobIds)), projectId, documentId)
	// NOTE: sqlite doesn't support an array type, so we can't use the native = ANY that we'd use in postgres. There are
	// technically sqlite extensions that provide this, but since this code is non-critical and mostly for tests and demos
	// we can just use the positional arg builder pattern.
	argsPositions := new(strings.Builder)
	for i, id := range blobIds {
		args = append(args, id)
		if i > 0 {
			argsPositions.WriteRune(',')
		}
		argsPositions.WriteRune('?')
	}
	if r, err := s.writer.ExecContext(ctx, `DELETE FROM blobs WHERE project_id = $1 AND document_id = $2 AND blob_id IN (`+argsPositions.String()+`)`, args...); err != nil {
		return fmt.Errorf("failed to perform delete blobs query: %w", err)
	} else if rc, _ := r.RowsAffected(); rc == 0 {
		return storage.ErrDocumentNotFound
	}
	return nil
}

var _ storage.BlobStorage = (*Storage)(nil)
