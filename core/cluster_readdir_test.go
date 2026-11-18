//go:build !windows

package core

import (
	"context"
	"sync/atomic"
	"syscall"

	"github.com/jacobsa/fuse/fuseops"
	. "gopkg.in/check.v1"

	"github.com/yandex-cloud/geesefs/core/cfg"
)

type ClusterReadDirTest struct{}

var _ = Suite(&ClusterReadDirTest{})

// A listing that fails inside ClusterFs.readDir's loop must come back as an
// error. It used to unlock dh.mu by hand and then again by its defer:
// "fatal error: sync: unlock of unlocked mutex", which no recover() stops,
// so a single failed ListObjects killed the mount.
func (s *ClusterReadDirTest) TestFailedListingInLoopReturnsAnError(t *C) {
	flags := cfg.DefaultFlags()
	flags.StatCacheTTL = 0 // no slurp, and every ReadDir checks for the next page
	var lists int32
	backend := &TestBackend{
		err: syscall.ENOSYS,
		ListBlobsFunc: func(param *ListBlobsInput) (*ListBlobsOutput, error) {
			if atomic.AddInt32(&lists, 1) == 1 {
				// Page 1 succeeds for loadChildren; page 2, fetched by ReadDir
				// inside readDir's loop once "a" is served, fails.
				return &ListBlobsOutput{
					IsTruncated:           true,
					NextContinuationToken: PString("page2"),
					Items:                 []BlobItemOutput{{Key: PString("a"), Size: 1}},
				}, nil
			}
			return nil, syscall.EIO
		},
	}
	fs, err := newGoofys(context.Background(), "test", flags, func(string, *cfg.FlagStorage) (StorageBackend, error) {
		return backend, nil
	})
	t.Assert(err, IsNil)
	cfs := &ClusterFs{Flags: flags, Goofys: fs}
	handle := fs.AddDirHandle(fs.inodes[fuseops.RootInodeID].OpenDir())

	dst := make([]byte, 4096)
	var n int
	err = cfs.readDir(handle, 0, dst, &n)
	t.Assert(err, NotNil)
	// Page 2 was attempted (with retries) inside the loop.
	t.Assert(atomic.LoadInt32(&lists) >= 2, Equals, true)

	// dh.mu is free again: a second call does not deadlock on it.
	n = 0
	err = cfs.readDir(handle, 0, dst, &n)
	t.Assert(err, NotNil)
}
