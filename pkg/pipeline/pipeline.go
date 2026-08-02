package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"file-watcher/pkg/archive"
	"file-watcher/pkg/namer"
	"file-watcher/pkg/pattern"
)

type Event struct {
	Path     string
	Basename string
	FileInfo os.FileInfo
}

type Processor func(ctx context.Context, ev Event) error
type Middleware func(next Processor) Processor

// BuildPipeline constructs the complete event processing pipeline middleware chain.
func BuildPipeline(destDir, matchPattern string, extractArchives bool, logger *slog.Logger) Processor {
	// Base handler: moves matched non-archive file to destination
	var baseProcessor Processor = func(ctx context.Context, ev Event) error {
		if _, err := os.Stat(ev.Path); os.IsNotExist(err) {
			return nil
		}

		targetName := namer.ResolveTarget(ev.Basename, func(name string) bool {
			_, err := os.Stat(filepath.Join(destDir, name))
			return err == nil
		})

		destPath := filepath.Join(destDir, targetName)

		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create dest directory: %w", err)
		}

		if err := os.Rename(ev.Path, destPath); err != nil {
			// Fallback to copy & remove if rename across devices fails
			if err := copyAndRemove(ev.Path, destPath); err != nil {
				if logger != nil {
					logger.Error("Failed to move file", "source", ev.Path, "dest", destPath, "error", err)
				}
				return err
			}
		}

		if logger != nil {
			logger.Info("Moved", "file", ev.Basename, "dest", destPath)
		}
		return nil
	}

	// Archive middleware
	archiveMiddleware := func(next Processor) Processor {
		return func(ctx context.Context, ev Event) error {
			if extractArchives && archive.IsArchive(ev.Path) {
				extracted, destPath, err := archive.InspectAndExtractSingleFile(ev.Path, matchPattern, destDir)
				if err != nil {
					if logger != nil {
						logger.Error("Failed processing archive", "archive", ev.Basename, "error", err)
					}
					return err
				}
				if extracted {
					if logger != nil {
						logger.Info("Extracted single archive file", "archive", ev.Basename, "dest", destPath)
					}
					return nil
				}
				return nil
			}
			return next(ctx, ev)
		}
	}

	// Pattern match middleware
	patternMiddleware := func(next Processor) Processor {
		return func(ctx context.Context, ev Event) error {
			matched, err := pattern.Match(matchPattern, ev.Basename)
			if err != nil {
				return err
			}
			if matched {
				return next(ctx, ev)
			}

			// If not matched, but it's an archive and archive extraction is enabled, pass through to archive middleware
			if extractArchives && archive.IsArchive(ev.Path) {
				return next(ctx, ev)
			}

			// Skip non-matching file
			return nil
		}
	}

	return patternMiddleware(archiveMiddleware(baseProcessor))
}

func copyAndRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := os.ReadDir(filepath.Dir(dst)); err != nil {
		// ignore
	}

	if _, err := ioCopy(out, in); err != nil {
		return err
	}

	_ = in.Close()
	_ = out.Close()

	return os.Remove(src)
}

func ioCopy(dst *os.File, src *os.File) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}
		}
		if er != nil {
			if er == os.ErrClosed || er.Error() == "EOF" {
				break
			}
			return written, er
		}
	}
	return written, nil
}
