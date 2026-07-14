package util

import (
	"fmt"
	"io"
	"math"
)

// ReadAllLimited reads all of r but fails if it would exceed max bytes, instead
// of buffering an unbounded amount — the OOM vector of a bare io.ReadAll on
// untrusted input (a request body, a fetched object, an attachment). A max <= 0
// means unlimited.
func ReadAllLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 || max == math.MaxInt64 {
		// max+1 would overflow int64 at MaxInt64; at that scale (~8 EiB) the cap
		// is unreachable, so treat it as unlimited.
		return io.ReadAll(r)
	}
	// Read one extra byte so a stream exactly at the limit is accepted, but any
	// overflow is detected without buffering the whole oversized input.
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("input exceeds the %d-byte limit", max)
	}
	return b, nil
}

// CheckInputSize returns an error if n exceeds max bytes; it is the in-memory
// counterpart of ReadAllLimited for input already held as a string or []byte. A
// max <= 0 means unlimited.
func CheckInputSize(label string, n, max int) error {
	if max > 0 && n > max {
		return fmt.Errorf("%s exceeds the %d-byte limit (%d bytes)", label, max, n)
	}
	return nil
}

// CappedWriter wraps an io.Writer and fails a Write that would push the total
// bytes written past its limit, so a bounded amount of output is produced from
// an otherwise-unbounded render or copy. A limit <= 0 means unlimited.
type CappedWriter struct {
	w       io.Writer
	limit   int
	written int
}

// NewCappedWriter returns a CappedWriter over w with the given byte limit
// (<= 0 means unlimited).
func NewCappedWriter(w io.Writer, limit int) *CappedWriter {
	return &CappedWriter{w: w, limit: limit}
}

// Write writes p to the underlying writer, or returns an error (writing nothing)
// if it would exceed the limit.
func (c *CappedWriter) Write(p []byte) (int, error) {
	// Compare as (remaining budget) vs len(p) so written+len(p) cannot overflow
	// int at the boundary and wrongly accept an over-limit write.
	if c.limit > 0 && len(p) > c.limit-c.written {
		return 0, fmt.Errorf("output exceeds the %d-byte limit", c.limit)
	}
	n, err := c.w.Write(p)
	c.written += n
	return n, err
}

// Written reports how many bytes have been written through the CappedWriter.
func (c *CappedWriter) Written() int { return c.written }
