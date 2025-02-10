/*
 * Copyright (c) 2023 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/PlakarKorp/plakar/objects"
	"github.com/PlakarKorp/plakar/snapshot/importer"
)

type S3Importer struct {
	minioClient *minio.Client
	bucket      string
	host        string
	scanDir     string

	ino uint64
}

func init() {
	importer.Register("s3", NewS3Importer)
}

func connect(location *url.URL) (*minio.Client, error) {
	endpoint := location.Host
	accessKeyID := location.User.Username()
	secretAccessKey, _ := location.User.Password()
	useSSL := false

	// Initialize minio client object.
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
}

func NewS3Importer(location string) (importer.Importer, error) {
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	conn, err := connect(parsed)
	if err != nil {
		return nil, err
	}

	atoms := strings.Split(parsed.RequestURI()[1:], "/")
	bucket := atoms[0]
	scanDir := filepath.Clean("/" + strings.Join(atoms[1:], "/"))

	return &S3Importer{
		bucket:      bucket,
		scanDir:     scanDir,
		minioClient: conn,
		host:        parsed.Host,
	}, nil
}

func (p *S3Importer) scanRecursive(prefix string, result chan importer.ScanResult) {
	for object := range p.minioClient.ListObjects(context.Background(), p.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: false}) {
		objectPath := "/" + object.Key
		if !strings.HasPrefix(objectPath, p.scanDir) && !strings.HasPrefix(p.scanDir, objectPath) {
			continue
		}

		if strings.HasSuffix(object.Key, "/") {
			p.scanRecursive(object.Key, result)
		} else {
			fi := objects.NewFileInfo(
				filepath.Base("/"+prefix+object.Key),
				object.Size,
				0700,
				object.LastModified,
				1,
				atomic.AddUint64(&p.ino, 1),
				0,
				0,
				0,
			)
			result <- importer.ScanRecord{Pathname: "/" + object.Key, FileInfo: fi}
		}
	}

	var currentName string
	if prefix == "" {
		currentName = "/"
	} else {
		currentName = filepath.Base(prefix)
	}

	result <- importer.ScanRecord{Pathname: "/" + prefix, FileInfo: objects.NewFileInfo(
		currentName,
		0,
		0700|os.ModeDir,
		time.Now(),
		0,
		atomic.AddUint64(&p.ino, 1),
		0,
		0,
		0,
	)}
}

func (p *S3Importer) Scan() (<-chan importer.ScanResult, error) {
	c := make(chan importer.ScanResult)
	go func() {
		defer close(c)
		p.scanRecursive("", c)
	}()
	return c, nil
}

func (p *S3Importer) NewReader(pathname string) (io.ReadCloser, error) {
	if pathname == "/" {
		return nil, fmt.Errorf("cannot read root directory")
	}
	if strings.HasSuffix(pathname, "/") {
		return nil, fmt.Errorf("cannot read directory")
	}
	pathname = strings.TrimPrefix(pathname, "/")

	obj, err := p.minioClient.GetObject(context.Background(), p.bucket, pathname,
		minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (p *S3Importer) Close() error {
	return nil
}

func (p *S3Importer) Root() string {
	return p.scanDir
}

func (p *S3Importer) Origin() string {
	return p.host + "/" + p.bucket
}

func (p *S3Importer) Type() string {
	return "s3"
}
