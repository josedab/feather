package cloudstorage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalBackend implements Backend using the local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocalBackend creates a new filesystem-backed storage backend.
func NewLocalBackend(basePath string) (*LocalBackend, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("creating base path %s: %w", basePath, err)
	}
	return &LocalBackend{basePath: basePath}, nil
}

func (l *LocalBackend) keyPath(key string) string {
	return filepath.Join(l.basePath, filepath.FromSlash(key))
}

// Put writes data to a file at the key path.
func (l *LocalBackend) Put(_ context.Context, key string, data []byte, contentType string, metadata map[string]string) error {
	path := l.keyPath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return nil
}

// Get reads a file and returns its data and metadata.
func (l *LocalBackend) Get(_ context.Context, key string) ([]byte, *ObjectInfo, error) {
	path := l.keyPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrObjectNotFound
		}
		return nil, nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	fi, _ := os.Stat(path)
	hash := sha256.Sum256(data)
	info := &ObjectInfo{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: "application/octet-stream",
		ETag:        fmt.Sprintf("%x", hash),
	}
	if fi != nil {
		info.LastModified = fi.ModTime()
	} else {
		info.LastModified = time.Now()
	}
	return data, info, nil
}

// GetRange reads a byte range from a file.
func (l *LocalBackend) GetRange(_ context.Context, key string, offset, length int64) ([]byte, error) {
	path := l.keyPath(key)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}
	defer f.Close()
	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("reading range: %w", err)
	}
	return buf[:n], nil
}

// Delete removes a file at the key path.
func (l *LocalBackend) Delete(_ context.Context, key string) error {
	path := l.keyPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("deleting file %s: %w", path, err)
	}
	return nil
}

// List walks the filesystem and returns objects matching the prefix.
func (l *LocalBackend) List(_ context.Context, prefix string, limit int) ([]ObjectInfo, error) {
	var results []ObjectInfo
	basePath := l.basePath
	err := filepath.Walk(basePath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(basePath, path)
		key := filepath.ToSlash(relPath)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		results = append(results, ObjectInfo{
			Key:          key,
			Size:         fi.Size(),
			ContentType:  "application/octet-stream",
			LastModified: fi.ModTime(),
		})
		if limit > 0 && len(results) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Key < results[j].Key
	})
	return results, nil
}

// Copy duplicates an object from srcKey to dstKey.
func (l *LocalBackend) Copy(ctx context.Context, srcKey, dstKey string) error {
	data, _, err := l.Get(ctx, srcKey)
	if err != nil {
		return err
	}
	return l.Put(ctx, dstKey, data, "", nil)
}

// Exists checks if a file exists at the key path.
func (l *LocalBackend) Exists(_ context.Context, key string) (bool, error) {
	path := l.keyPath(key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Close is a no-op for the local backend.
func (l *LocalBackend) Close() error {
	return nil
}
