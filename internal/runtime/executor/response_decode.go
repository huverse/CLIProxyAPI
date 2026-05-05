package executor

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

const compressedAcceptEncoding = "gzip, deflate, br, zstd"

type compositeReadCloser struct {
	io.Reader
	closers []func() error
}

func (c *compositeReadCloser) Close() error {
	var firstErr error
	for i := range c.closers {
		if c.closers[i] == nil {
			continue
		}
		if err := c.closers[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// peekableBody wraps a bufio.Reader around the original ReadCloser so magic
// bytes can be inspected without consuming them from the stream.
type peekableBody struct {
	*bufio.Reader
	closer io.Closer
}

func (p *peekableBody) Close() error {
	return p.closer.Close()
}

func decodeHTTPResponseBody(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}
	if resp.Body == nil {
		return fmt.Errorf("response body is nil")
	}
	decodedBody, err := decodeResponseBody(resp.Body, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return err
	}
	resp.Body = decodedBody
	if resp.Header.Get("Content-Encoding") != "" {
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
	}
	return nil
}

func decodeResponseBody(body io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	if body == nil {
		return nil, fmt.Errorf("response body is nil")
	}
	encodings := parseContentEncodings(contentEncoding)
	if len(encodings) == 0 {
		return decodeByMagicBytes(body)
	}
	for i := len(encodings) - 1; i >= 0; i-- {
		var err error
		body, err = wrapDecodedBody(body, encodings[i])
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func parseContentEncodings(contentEncoding string) []string {
	if contentEncoding == "" {
		return nil
	}
	parts := strings.Split(contentEncoding, ",")
	encodings := make([]string, 0, len(parts))
	for _, raw := range parts {
		encoding := strings.TrimSpace(strings.ToLower(raw))
		switch encoding {
		case "", "identity":
			continue
		default:
			encodings = append(encodings, encoding)
		}
	}
	return encodings
}

func decodeByMagicBytes(body io.ReadCloser) (io.ReadCloser, error) {
	pb := &peekableBody{Reader: bufio.NewReader(body), closer: body}
	magic, peekErr := pb.Peek(4)
	if peekErr != nil && !(peekErr == io.EOF && len(magic) >= 2) {
		return pb, nil
	}
	switch {
	case len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		gzipReader, err := gzip.NewReader(pb)
		if err != nil {
			_ = pb.Close()
			return nil, fmt.Errorf("magic-byte gzip: failed to create reader: %w", err)
		}
		return &compositeReadCloser{
			Reader: gzipReader,
			closers: []func() error{
				gzipReader.Close,
				pb.Close,
			},
		}, nil
	case len(magic) >= 4 && magic[0] == 0x28 && magic[1] == 0xb5 && magic[2] == 0x2f && magic[3] == 0xfd:
		decoder, err := zstd.NewReader(pb)
		if err != nil {
			_ = pb.Close()
			return nil, fmt.Errorf("magic-byte zstd: failed to create reader: %w", err)
		}
		return &compositeReadCloser{
			Reader: decoder,
			closers: []func() error{
				func() error { decoder.Close(); return nil },
				pb.Close,
			},
		}, nil
	default:
		return pb, nil
	}
}

func wrapDecodedBody(body io.ReadCloser, encoding string) (io.ReadCloser, error) {
	switch encoding {
	case "gzip", "x-gzip":
		gzipReader, err := gzip.NewReader(body)
		if err != nil {
			_ = body.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &compositeReadCloser{
			Reader: gzipReader,
			closers: []func() error{
				gzipReader.Close,
				func() error { return body.Close() },
			},
		}, nil
	case "deflate":
		return newDeflateReadCloser(body)
	case "br":
		return &compositeReadCloser{
			Reader: brotli.NewReader(body),
			closers: []func() error{
				func() error { return body.Close() },
			},
		}, nil
	case "zstd", "zst":
		decoder, err := zstd.NewReader(body)
		if err != nil {
			_ = body.Close()
			return nil, fmt.Errorf("failed to create zstd reader: %w", err)
		}
		return &compositeReadCloser{
			Reader: decoder,
			closers: []func() error{
				func() error { decoder.Close(); return nil },
				func() error { return body.Close() },
			},
		}, nil
	default:
		_ = body.Close()
		return nil, fmt.Errorf("unsupported content encoding %q", encoding)
	}
}

func newDeflateReadCloser(body io.ReadCloser) (io.ReadCloser, error) {
	pb := &peekableBody{Reader: bufio.NewReader(body), closer: body}
	if looksLikeZlib(pb) {
		zlibReader, err := zlib.NewReader(pb)
		if err != nil {
			_ = pb.Close()
			return nil, fmt.Errorf("failed to create zlib deflate reader: %w", err)
		}
		return &compositeReadCloser{
			Reader: zlibReader,
			closers: []func() error{
				zlibReader.Close,
				pb.Close,
			},
		}, nil
	}
	deflateReader := flate.NewReader(pb)
	return &compositeReadCloser{
		Reader: deflateReader,
		closers: []func() error{
			deflateReader.Close,
			pb.Close,
		},
	}, nil
}

func looksLikeZlib(r interface{ Peek(int) ([]byte, error) }) bool {
	header, err := r.Peek(2)
	if err != nil {
		return false
	}
	cmf, flg := int(header[0]), int(header[1])
	return cmf&0x0f == 8 && cmf>>4 <= 7 && (cmf*256+flg)%31 == 0
}
