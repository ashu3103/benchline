package parser

import (
	"io"
	"bytes"
)

type ChunkReader struct {
	src     io.Reader
	bufSize int
	carry   []byte // unfinished line left over from the previous chunk
	chunk   []byte // current window into the source
	pos     int    // read cursor inside chunk
	eof     bool
}

func NewChunkedReader(r io.Reader, bufSize int) *ChunkReader {
	if bufSize <= 0 {
		bufSize = 4096
	}
	return &ChunkReader{src: r, bufSize: bufSize}
}


// Read satisfies io.Reader. It fills p with data assembled from fixed-size
// chunks of the underlying source, prepending any carry-over from the previous
// call. Splitting always happens at the last newline so lines remain intact.
func (c *ChunkReader) Read(p []byte) (int, error) {
	// If the current chunk still has unread bytes, serve from it first.
	if c.pos < len(c.chunk) {
		n := copy(p, c.chunk[c.pos:])
		c.pos += n
		return n, nil
	}
 
	if c.eof {
		// Drain any remaining carry.
		if len(c.carry) > 0 {
			n := copy(p, c.carry)
			c.carry = c.carry[n:]
			return n, nil
		}
		return 0, io.EOF
	}
 
	// Read the next raw chunk from the source.
	raw := make([]byte, c.bufSize)
	n, err := c.src.Read(raw)
	if err == io.EOF {
		c.eof = true
	} else if err != nil {
		return 0, err
	}
	raw = raw[:n]
 
	// Prepend carry-over from previous chunk.
	combined := append(c.carry, raw...)
	c.carry = nil
 
	// Find the last newline so we never hand p a partial line.
	lastNL := bytes.LastIndexByte(combined, '\n')
	if lastNL == -1 {
		if c.eof {
			// No newline and no more data — flush everything.
			c.chunk = combined
		} else {
			// Entire chunk is one partial line; carry it forward.
			c.carry = combined
			c.chunk = nil
		}
	} else {
		// Keep complete lines; carry the incomplete tail.
		c.chunk = combined[:lastNL+1]
		c.carry = combined[lastNL+1:]
	}
 
	c.pos = 0
	return c.Read(p) // recurse once to serve from the freshly filled chunk
}