package archive

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

type tarIterator struct {
	file   *os.File
	decomp io.Reader
	tr     *tar.Reader
}

func (ti *tarIterator) Next() (*entryItem, io.Reader, error) {
	for {
		hdr, err := ti.tr.Next()
		if err != nil {
			return nil, nil, err
		}
		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/") {
			continue
		}
		return &entryItem{Name: hdr.Name}, ti.tr, nil
	}
}

func (ti *tarIterator) Close() error {
	closeDecompressor(ti.decomp)
	return ti.file.Close()
}

// inspectAndExtractTar handles two-pass streaming inspection and extraction of tar-based archives (.tar, .tar.gz, .tar.bz2, .tar.xz).
func inspectAndExtractTar(archivePath, targetPattern, destDir string) (bool, string, error) {
	return inspectAndExtractSequential(archivePath, targetPattern, destDir, func() (archiveIterator, error) {
		file, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}

		decomp, err := openDecompressor(file, archivePath)
		if err != nil {
			_ = file.Close()
			return nil, err
		}

		return &tarIterator{
			file:   file,
			decomp: decomp,
			tr:     tar.NewReader(decomp),
		}, nil
	})
}

// openDecompressor wraps r in a decompression reader (gzip, bzip2, xz) based on the archive file extension.
func openDecompressor(r io.Reader, path string) (io.Reader, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return gzip.NewReader(r)
	}
	if strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz") || strings.HasSuffix(lower, ".tbz2") {
		return bzip2.NewReader(r), nil
	}
	if strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz") {
		return xz.NewReader(r)
	}
	return r, nil
}

// closeDecompressor closes r if it implements io.Closer.
func closeDecompressor(r io.Reader) {
	if cl, ok := r.(io.Closer); ok {
		_ = cl.Close()
	}
}
