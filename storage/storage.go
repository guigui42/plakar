/*
 * Copyright (c) 2021 Gilles Chehade <gilles@poolp.org>
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

package storage

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PlakarKorp/plakar/chunking"
	"github.com/PlakarKorp/plakar/compression"
	"github.com/PlakarKorp/plakar/encryption"
	"github.com/PlakarKorp/plakar/hashing"
	"github.com/PlakarKorp/plakar/objects"
	"github.com/PlakarKorp/plakar/packfile"
	"github.com/PlakarKorp/plakar/resources"
	"github.com/PlakarKorp/plakar/versioning"
	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
)

const VERSION string = "1.0.0"

func init() {
	versioning.Register(resources.RT_CONFIG, versioning.FromString(VERSION))
}

type Configuration struct {
	Version      versioning.Version `msgpack:"-"`
	Timestamp    time.Time
	RepositoryID uuid.UUID

	Packfile    packfile.Configuration
	Chunking    chunking.Configuration
	Hashing     hashing.Configuration
	Compression *compression.Configuration
	Encryption  *encryption.Configuration
}

func NewConfiguration() *Configuration {
	return &Configuration{
		Version:      versioning.FromString(VERSION),
		Timestamp:    time.Now(),
		RepositoryID: uuid.Must(uuid.NewRandom()),

		Packfile: *packfile.NewDefaultConfiguration(),
		Chunking: *chunking.NewDefaultConfiguration(),
		Hashing:  *hashing.NewDefaultConfiguration(),

		Compression: compression.NewDefaultConfiguration(),
		Encryption:  encryption.NewDefaultConfiguration(),
	}
}

func NewConfigurationFromBytes(version versioning.Version, data []byte) (*Configuration, error) {
	var configuration Configuration
	err := msgpack.Unmarshal(data, &configuration)
	if err != nil {
		return nil, err
	}
	configuration.Version = version
	return &configuration, nil
}

func NewConfigurationFromWrappedBytes(data []byte) (*Configuration, error) {
	var configuration Configuration

	version := versioning.Version(binary.LittleEndian.Uint32(data[12:16]))

	data = data[:len(data)-int(STORAGE_FOOTER_SIZE)]
	data = data[STORAGE_HEADER_SIZE:]

	err := msgpack.Unmarshal(data, &configuration)
	if err != nil {
		return nil, err
	}
	configuration.Version = version
	return &configuration, nil
}

func (c *Configuration) ToBytes() ([]byte, error) {
	return msgpack.Marshal(c)
}

type Store interface {
	Create(repository string, config []byte) error
	Open(repository string) ([]byte, error)
	Location() string

	GetStates() ([]objects.Checksum, error)
	PutState(checksum objects.Checksum, rd io.Reader) error
	GetState(checksum objects.Checksum) (io.Reader, error)
	DeleteState(checksum objects.Checksum) error

	GetPackfiles() ([]objects.Checksum, error)
	PutPackfile(checksum objects.Checksum, rd io.Reader) error
	GetPackfile(checksum objects.Checksum) (io.Reader, error)
	GetPackfileBlob(checksum objects.Checksum, offset uint64, length uint32) (io.Reader, error)
	DeletePackfile(checksum objects.Checksum) error

	Close() error
}

var muBackends sync.Mutex
var backends map[string]func(string) Store = make(map[string]func(string) Store)

func NewStore(name string, location string) (Store, error) {
	muBackends.Lock()
	defer muBackends.Unlock()

	if backend, exists := backends[name]; !exists {
		return nil, fmt.Errorf("backend '%s' does not exist", name)
	} else {
		backendInstance := backend(location)
		return backendInstance, nil
	}
}

func Register(name string, backend func(string) Store) {
	muBackends.Lock()
	defer muBackends.Unlock()

	if _, ok := backends[name]; ok {
		log.Fatalf("backend '%s' registered twice", name)
	}
	backends[name] = backend
}

func Backends() []string {
	muBackends.Lock()
	defer muBackends.Unlock()

	ret := make([]string, 0)
	for backendName := range backends {
		ret = append(ret, backendName)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i] < ret[j]
	})
	return ret
}

func New(location string) (Store, error) {
	backendName := "fs"
	if !strings.HasPrefix(location, "/") {
		if strings.HasPrefix(location, "tcp://") || strings.HasPrefix(location, "ssh://") || strings.HasPrefix(location, "stdio://") {
			backendName = "plakard"
		} else if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
			backendName = "http"
		} else if strings.HasPrefix(location, "sqlite://") {
			backendName = "database"
		} else if strings.HasPrefix(location, "s3://") {
			backendName = "s3"
		} else if strings.HasPrefix(location, "null://") {
			backendName = "null"
		} else if strings.HasPrefix(location, "fs://") {
			backendName = "fs"
		} else if strings.Contains(location, "://") {
			return nil, fmt.Errorf("unsupported plakar protocol")
		}
	}

	if backendName == "fs" && !strings.HasPrefix(location, "/") {
		if !strings.HasPrefix(location, "fs://") {
			tmp, err := filepath.Abs(location)
			if err != nil {
				return nil, err
			}
			location = tmp
		}
	}
	return NewStore(backendName, location)
}

func Open(location string) (Store, []byte, error) {
	store, err := New(location)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", flag.CommandLine.Name(), err)
		return nil, nil, err
	}

	serializedConfig, err := store.Open(location)
	if err != nil {
		return nil, nil, err
	}

	return store, serializedConfig, nil
}

func Create(location string, configuration []byte) (Store, error) {
	store, err := New(location)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", flag.CommandLine.Name(), err)
		return nil, err
	}

	if err = store.Create(location, configuration); err != nil {
		return nil, err
	} else {
		return store, nil
	}
}
