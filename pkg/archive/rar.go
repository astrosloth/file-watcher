package archive

import (
	"fmt"
	"io"

	"github.com/nwaples/rardecode/v2"
)

type rarIterator struct {
	rr *rardecode.ReadCloser
}

func (ri *rarIterator) Next() (*entryItem, io.Reader, error) {
	for {
		hdr, err := ri.rr.Next()
		if err != nil {
			return nil, nil, err
		}
		if hdr.IsDir {
			continue
		}
		return &entryItem{Name: hdr.Name}, ri.rr, nil
	}
}

func (ri *rarIterator) Close() error {
	return ri.rr.Close()
}

// inspectAndExtractRar handles two-pass streaming inspection and extraction of RAR archives.
func inspectAndExtractRar(archivePath, targetPattern, destDir string) (bool, string, error) {
	return inspectAndExtractSequential(archivePath, targetPattern, destDir, func() (archiveIterator, error) {
		rr, err := rardecode.OpenReader(archivePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open rar: %w", err)
		}
		return &rarIterator{rr: rr}, nil
	})
}
