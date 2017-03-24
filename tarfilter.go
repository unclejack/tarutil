package tarutil

import (
	"archive/tar"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type TarFilter interface {
	SetTarWriter(tw *tar.Writer) error
	HandleEntry(*tar.Header) (bool, bool, error)
	Close() error
}

func FilterTarUsingFilter(r io.Reader, f TarFilter) (io.Reader, error) {
	var (
		pr, pw      = io.Pipe()
		tr          = tar.NewReader(r)
		tw          = tar.NewWriter(pw)
		writeData   bool
		writeHeader bool
	)

	if err := f.SetTarWriter(tw); err != nil {
		return nil, err
	}
	go func() {
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				f.Close()
				tw.Close()
				break
			}

			if err != nil {
				pw.Close()
				break
			}

			writeData, writeHeader, err = f.HandleEntry(hdr)
			if err != nil {
				pw.Close()
				break
			}

			if !writeHeader {
				continue
			}
			err = tw.WriteHeader(hdr)
			if err != nil {
				pw.Close()
				break
			}

			if !writeData || hdr.Size == 0 {
				continue
			}

			_, err = io.Copy(tw, tr)
			if err != nil {
				pw.Close()
				break
			}
		}
	}()
	return pr, nil
}

type OverlayWhiteouts struct {
	dirs map[string]*tar.Header
	tw   *tar.Writer
}

func NewOverlayWhiteouts() *OverlayWhiteouts {
	return &OverlayWhiteouts{
		dirs: make(map[string]*tar.Header),
	}

}

func (o *OverlayWhiteouts) SetTarWriter(tw *tar.Writer) error {
	if o.tw == nil {
		o.tw = tw
		return nil
	}
	return fmt.Errorf("the TarWriter is already set")
}

func (o *OverlayWhiteouts) Close() error {
	if o.tw == nil {
		return fmt.Errorf("the tarWriter isn't set")
	}
	for k, h := range o.dirs {
		if err := o.tw.WriteHeader(h); err != nil {
			return err
		}
		delete(o.dirs, k)
	}
	return nil
}

func (o *OverlayWhiteouts) HandleEntry(h *tar.Header) (bool, bool, error) {
	if o.tw == nil {
		return false, false, fmt.Errorf("the tarWriter isn't set")
	}
	name := filepath.Clean(h.Name)
	base := filepath.Clean(filepath.Base(name))
	dir := filepath.Dir(name)

	if h.Typeflag == tar.TypeDir {
		o.dirs[base] = h
		return false, false, nil
	}

	if dirHeader, ok := o.dirs[dir]; ok {
		delete(o.dirs, dir)
		if base == whiteoutOpaqueDir {
			h.Xattrs["trusted.overlay.opaque"] = "y"
			return false, true, nil
		}
		if err := o.tw.WriteHeader(dirHeader); err != nil {
			return false, false, err
		}

	}

	if strings.HasPrefix(base, whiteoutPrefix) {
		convertWhiteoutToOverlay(h, dir, base)
		return false, true, nil
	}
	return true, true, nil
}

func convertWhiteoutToOverlay(h *tar.Header, dir, base string) {
	originalBase := base[len(whiteoutPrefix):]
	originalPath := filepath.Join(dir, originalBase)
	h.Typeflag = tar.TypeChar
	h.Name = originalPath
}