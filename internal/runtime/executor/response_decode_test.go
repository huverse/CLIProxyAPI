package executor

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func TestDecodeResponseBodySupportedEncodings(t *testing.T) {
	const plaintext = `{"ok":true}`

	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
	}{
		{
			name:            "identity",
			contentEncoding: "",
			body:            []byte(plaintext),
		},
		{
			name:            "gzip",
			contentEncoding: "gzip",
			body:            gzipBytes(t, []byte(plaintext)),
		},
		{
			name:            "zlib deflate",
			contentEncoding: "deflate",
			body:            zlibDeflateBytes(t, []byte(plaintext)),
		},
		{
			name:            "raw deflate",
			contentEncoding: "deflate",
			body:            rawDeflateBytes(t, []byte(plaintext)),
		},
		{
			name:            "brotli",
			contentEncoding: "br",
			body:            brotliBytes(t, []byte(plaintext)),
		},
		{
			name:            "zstd",
			contentEncoding: "zstd",
			body:            zstdBytes(t, []byte(plaintext)),
		},
		{
			name:            "multi layer",
			contentEncoding: "gzip, br",
			body:            brotliBytes(t, gzipBytes(t, []byte(plaintext))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeResponseBody(io.NopCloser(bytes.NewReader(tt.body)), tt.contentEncoding)
			if err != nil {
				t.Fatalf("decodeResponseBody error: %v", err)
			}
			defer decoded.Close()
			got, err := io.ReadAll(decoded)
			if err != nil {
				t.Fatalf("ReadAll error: %v", err)
			}
			if string(got) != plaintext {
				t.Fatalf("decoded = %q, want %q", got, plaintext)
			}
		})
	}
}

func TestDecodeHTTPResponseBodyStripsCompressionHeaders(t *testing.T) {
	resp := &http.Response{
		Header:        http.Header{"Content-Encoding": {"zstd"}, "Content-Length": {"123"}},
		Body:          io.NopCloser(bytes.NewReader(zstdBytes(t, []byte(`{"ok":true}`)))),
		ContentLength: 123,
	}

	if err := decodeHTTPResponseBody(resp); err != nil {
		t.Fatalf("decodeHTTPResponseBody error: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Fatalf("decoded = %q", got)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want empty", got)
	}
	if got := resp.Header.Get("Content-Length"); got != "" {
		t.Fatalf("Content-Length = %q, want empty", got)
	}
	if resp.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", resp.ContentLength)
	}
}

func TestDecodeResponseBodyUnsupportedEncodingFailsClearly(t *testing.T) {
	_, err := decodeResponseBody(io.NopCloser(strings.NewReader("payload")), "compress")
	if err == nil {
		t.Fatal("expected unsupported encoding error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported content encoding "compress"`) {
		t.Fatalf("error = %v", err)
	}
}

func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("gzip Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

func zlibDeflateBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zlib Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zlib Close: %v", err)
	}
	return buf.Bytes()
}

func rawDeflateBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("flate NewWriter: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("flate Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate Close: %v", err)
	}
	return buf.Bytes()
}

func brotliBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("brotli Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("brotli Close: %v", err)
	}
	return buf.Bytes()
}

func zstdBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd NewWriter: %v", err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("zstd Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zstd Close: %v", err)
	}
	return buf.Bytes()
}
