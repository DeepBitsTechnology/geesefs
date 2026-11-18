package core

import (
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
