package core

import (
	"sync"

	"github.com/jacobsa/fuse/fuseutil"
	. "gopkg.in/check.v1"

	"github.com/yandex-cloud/geesefs/core/cfg"
)

type DirentTest struct{}

var _ = Suite(&DirentTest{})

func direntInode(fs *Goofys, size uint64, meta map[string][]byte) *Inode {
	return &Inode{
		Name:         "x",
		fs:           fs,
		Attributes:   InodeAttributes{Size: size, Mode: fs.flags.FileMode},
		userMetadata: meta,
	}
}

// readdir's d_type must agree with the mode GetAttr reports (DeepBits ADR-0008).
func (s *DirentTest) TestDirentTypeMatchesInflatedMode(t *C) {
	fs := &Goofys{flags: cfg.DefaultFlags()}
	link := direntInode(fs, 0, map[string][]byte{fs.flags.SymlinkAttr: []byte("busybox")})
	file := direntInode(fs, 0, map[string][]byte{})
	unfilled := direntInode(fs, 0, nil)
	sized := direntInode(fs, 42, nil)
	dir := direntInode(fs, 0, nil)
	dir.dir = &DirInodeData{}

	// A symlink whose target is known (HEADed, or created here) is DT_Link,
	// not DT_File: the case that made os.scandir and lstat disagree.
	t.Assert(makeDirEntry(link, 5).Type, Equals, fuseutil.DT_Link)
	t.Assert(link.InflateAttributes().Mode.IsRegular(), Equals, false)

	t.Assert(makeDirEntry(file, 5).Type, Equals, fuseutil.DT_File)
	t.Assert(makeDirEntry(sized, 5).Type, Equals, fuseutil.DT_File)
	// Not HEADed yet: unknown, so callers lstat it.
	t.Assert(makeDirEntry(unfilled, 5).Type, Equals, fuseutil.DT_Unknown)
	t.Assert(makeDirEntry(dir, 5).Type, Equals, fuseutil.DT_Directory)
}

// Cluster mode's readdir built dirents without the inode lock while a LookUp
// could be filling userMetadata (fillXattr): a concurrent map read and write,
// which the Go runtime aborts on. Run with -race.
func (s *DirentTest) TestLockedDirEntryHoldsTheInodeLock(t *C) {
	fs := &Goofys{flags: cfg.DefaultFlags()}
	inode := direntInode(fs, 0, map[string][]byte{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			inode.mu.Lock()
			inode.userMetadata[fs.flags.SymlinkAttr] = []byte("busybox")
			delete(inode.userMetadata, fs.flags.SymlinkAttr)
			inode.mu.Unlock()
		}
	}()
	for i := 0; i < 2000; i++ {
		_ = lockedDirEntry(inode, 5)
	}
	wg.Wait()
	inode.userMetadata[fs.flags.SymlinkAttr] = []byte("busybox")
	t.Assert(lockedDirEntry(inode, 5).Type, Equals, fuseutil.DT_Link)
}
