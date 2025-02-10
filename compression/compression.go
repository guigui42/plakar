package compression

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

type Configuration struct {
	Algorithm  string
	Level      int  // Compression level (-1 for default)
	WindowSize int  // Window size for algorithms like zstd or Brotli
	ChunkSize  int  // Chunk size for streaming compression
	BlockSize  int  // Block size for block-based algorithms like bzip2
	EnableCRC  bool // Enable/disable checksum (e.g., gzip CRC32, zstd)
}

func NewDefaultConfiguration() *Configuration {
	return &Configuration{
		Algorithm:  "LZ4",
		Level:      int(lz4.Level9),
		WindowSize: -1,
		ChunkSize:  -1,
		BlockSize:  -1,
		EnableCRC:  false,
	}
}

func LookupDefaultConfiguration(algorithm string) (*Configuration, error) {
	switch algorithm {
	case "LZ4":
		return &Configuration{
			Algorithm:  "LZ4",
			Level:      int(lz4.Level9),
			WindowSize: -1,
			ChunkSize:  -1,
			BlockSize:  -1,
			EnableCRC:  false,
		}, nil
	case "GZIP":
		return &Configuration{
			Algorithm:  "GZIP",
			Level:      -1,
			WindowSize: -1,
			ChunkSize:  -1,
			BlockSize:  -1,
			EnableCRC:  false,
		}, nil
	default:
		return nil, fmt.Errorf("unknown hashing algorithm: %s", algorithm)
	}
}

func DeflateStream(name string, r io.Reader) (io.Reader, error) {
	m := map[string]func(io.Reader) (io.Reader, error){
		"GZIP": DeflateGzipStream,
		"LZ4":  DeflateLZ4Stream,
	}
	if fn, exists := m[name]; exists {
		return fn(r)
	}
	return nil, fmt.Errorf("unsupported compression method %q", name)
}

func DeflateGzipStream(r io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		gw := gzip.NewWriter(pw)
		defer pw.Close()
		defer gw.Close()

		_, err := io.Copy(gw, r)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func DeflateLZ4Stream(r io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		lw := lz4.NewWriter(pw)
		defer pw.Close()
		defer lw.Close()
		_, err := io.Copy(lw, r)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func InflateStream(name string, r io.Reader) (io.Reader, error) {
	m := map[string]func(io.Reader) (io.Reader, error){
		"GZIP": InflateGzipStream,
		"LZ4":  InflateLZ4Stream,
	}
	if fn, exists := m[name]; exists {
		return fn(r)
	}
	return nil, fmt.Errorf("unsupported compression method %q", name)
}

func InflateGzipStream(r io.Reader) (io.Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer gz.Close()

		_, err := io.Copy(pw, gz)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func InflateLZ4Stream(r io.Reader) (io.Reader, error) {
	pr, pw := io.Pipe()
	go func() {
		lz := lz4.NewReader(r)
		defer pw.Close()

		_, err := io.Copy(pw, lz)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()
	return pr, nil
}
