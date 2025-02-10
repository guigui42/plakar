package snapshot

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"sync"

	"github.com/PlakarKorp/plakar/events"
	"github.com/PlakarKorp/plakar/resources"
	"github.com/PlakarKorp/plakar/snapshot/vfs"
)

type CheckOptions struct {
	MaxConcurrency uint64
	FastCheck      bool
}

func snapshotCheckPath(snap *Snapshot, fsc *vfs.Filesystem, pathname string, opts *CheckOptions, concurrency chan bool, wg *sync.WaitGroup) (bool, error) {
	snap.Event(events.PathEvent(snap.Header.Identifier, pathname))
	file, err := fsc.GetEntry(pathname)
	if err != nil {
		snap.Event(events.DirectoryMissingEvent(snap.Header.Identifier, pathname))
		return false, err
	}

	if file.Stat().Mode().IsDir() {
		dir := file.Open(fsc, pathname).(fs.ReadDirFile)
		defer dir.Close()

		snap.Event(events.DirectoryEvent(snap.Header.Identifier, pathname))
		for {
			entries, err := dir.ReadDir(16)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				snap.Event(events.DirectoryCorruptedEvent(snap.Header.Identifier, pathname))
				return false, err
			}
			for i := range entries {
				ok, err := snapshotCheckPath(snap, fsc, path.Join(pathname, entries[i].Name()),
					opts, concurrency, wg)
				if err != nil {
					snap.Event(events.DirectoryCorruptedEvent(snap.Header.Identifier, pathname))
					return ok, err
				}
			}
		}
		snap.Event(events.DirectoryOKEvent(snap.Header.Identifier, pathname))
		return true, nil
	}

	if !file.Stat().Mode().IsRegular() {
		return true, nil
	}

	snap.Event(events.FileEvent(snap.Header.Identifier, pathname))
	concurrency <- true
	wg.Add(1)
	go func(_fileEntry *vfs.Entry) {
		defer wg.Done()
		defer func() { <-concurrency }()

		object, err := snap.LookupObject(_fileEntry.Object)
		if err != nil {
			snap.Event(events.ObjectMissingEvent(snap.Header.Identifier, _fileEntry.Object))
			return
		}

		hasher := snap.repository.GetMACHasher()
		snap.Event(events.ObjectEvent(snap.Header.Identifier, object.MAC))
		complete := true
		for _, chunk := range object.Chunks {
			snap.Event(events.ChunkEvent(snap.Header.Identifier, chunk.MAC))
			if opts.FastCheck {
				exists := snap.BlobExists(resources.RT_CHUNK, chunk.MAC)
				if !exists {
					snap.Event(events.ChunkMissingEvent(snap.Header.Identifier, chunk.MAC))
					complete = false
					break
				}
				snap.Event(events.ChunkOKEvent(snap.Header.Identifier, chunk.MAC))
			} else {
				exists := snap.BlobExists(resources.RT_CHUNK, chunk.MAC)
				if !exists {
					snap.Event(events.ChunkMissingEvent(snap.Header.Identifier, chunk.MAC))
					complete = false
					break
				}
				data, err := snap.GetBlob(resources.RT_CHUNK, chunk.MAC)
				if err != nil {
					snap.Event(events.ChunkMissingEvent(snap.Header.Identifier, chunk.MAC))
					complete = false
					break
				}
				snap.Event(events.ChunkOKEvent(snap.Header.Identifier, chunk.MAC))

				hasher.Write(data)

				checksum := snap.repository.ComputeMAC(data)
				if !bytes.Equal(checksum[:], chunk.MAC[:]) {
					snap.Event(events.ChunkCorruptedEvent(snap.Header.Identifier, chunk.MAC))
					complete = false
					break
				}
			}
		}
		if !complete {
			snap.Event(events.ObjectCorruptedEvent(snap.Header.Identifier, object.MAC))
		} else {
			snap.Event(events.ObjectOKEvent(snap.Header.Identifier, object.MAC))
		}

		if !bytes.Equal(hasher.Sum(nil), object.MAC[:]) {
			snap.Event(events.ObjectCorruptedEvent(snap.Header.Identifier, object.MAC))
			snap.Event(events.FileCorruptedEvent(snap.Header.Identifier, pathname))
			return
		}
	}(file)
	snap.Event(events.FileOKEvent(snap.Header.Identifier, pathname, file.Size()))
	return true, nil
}

func (snap *Snapshot) Check(pathname string, opts *CheckOptions) (bool, error) {
	snap.Event(events.StartEvent())
	defer snap.Event(events.DoneEvent())

	fs, err := snap.Filesystem()
	if err != nil {
		return false, err
	}

	maxConcurrency := opts.MaxConcurrency
	if maxConcurrency == 0 {
		maxConcurrency = uint64(snap.AppContext().MaxConcurrency)
	}

	maxConcurrencyChan := make(chan bool, maxConcurrency)
	wg := sync.WaitGroup{}
	defer wg.Wait()
	defer close(maxConcurrencyChan)

	return snapshotCheckPath(snap, fs, pathname, opts, maxConcurrencyChan, &wg)
}
