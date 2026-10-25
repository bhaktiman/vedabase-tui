package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// get downloads url into memory, failing when it exceeds limit bytes.
// progress, when non-nil, is called as bytes arrive.
func (u *Updater) get(ctx context.Context, url string, limit int64, progress func(done, total int64)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: %s: %s", url, resp.Status)
	}
	total := max(resp.ContentLength, 0)
	if total > limit {
		return nil, fmt.Errorf("update: %s: larger than %d bytes", url, limit)
	}
	var buf []byte
	chunk := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if int64(len(buf)) > limit {
			return nil, fmt.Errorf("update: %s: larger than %d bytes", url, limit)
		}
		if n > 0 && progress != nil {
			progress(int64(len(buf)), total)
		}
		if err == io.EOF {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
	}
}
