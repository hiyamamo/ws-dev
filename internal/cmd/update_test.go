package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestAssetName(t *testing.T) {
	cases := []struct {
		tag, goos, goarch, want string
	}{
		{"v0.2.0", "darwin", "arm64", "ws-dev_0.2.0_darwin_arm64.tar.gz"},
		{"0.2.0", "linux", "amd64", "ws-dev_0.2.0_linux_amd64.tar.gz"},
	}
	for _, c := range cases {
		if got := assetName(c.tag, c.goos, c.goarch); got != c.want {
			t.Errorf("assetName(%q,%q,%q) = %q, want %q", c.tag, c.goos, c.goarch, got, c.want)
		}
	}
}

func TestSameVersion(t *testing.T) {
	yes := [][2]string{{"0.2.0", "v0.2.0"}, {"v0.2.0", "0.2.0"}, {"v1.2.3", "v1.2.3"}}
	for _, p := range yes {
		if !sameVersion(p[0], p[1]) {
			t.Errorf("sameVersion(%q,%q) = false, want true", p[0], p[1])
		}
	}
	no := [][2]string{{"0.2.0", "v0.2.1"}, {"dev", "v0.2.0"}}
	for _, p := range no {
		if sameVersion(p[0], p[1]) {
			t.Errorf("sameVersion(%q,%q) = true, want false", p[0], p[1])
		}
	}
}

func TestParseChecksum(t *testing.T) {
	data := []byte("aaa111  ws-dev_0.2.0_linux_amd64.tar.gz\nbbb222  ws-dev_0.2.0_darwin_arm64.tar.gz\n")
	got, ok := parseChecksum(data, "ws-dev_0.2.0_darwin_arm64.tar.gz")
	if !ok || got != "bbb222" {
		t.Errorf("parseChecksum = %q,%v want bbb222,true", got, ok)
	}
	if _, ok := parseChecksum(data, "missing.tar.gz"); ok {
		t.Error("parseChecksum should miss unknown file")
	}
}

func TestExtractBinary(t *testing.T) {
	// Build an in-memory tar.gz containing the binary plus extra files.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entries := map[string]string{
		"README.md": "readme",
		"ws-dev":    "BINARY-CONTENT",
		"LICENSE":   "license",
	}
	for name, body := range entries {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.Close()
	_ = gz.Close()

	got, err := extractBinary(buf.Bytes(), "ws-dev")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "BINARY-CONTENT" {
		t.Errorf("extractBinary = %q", got)
	}

	if _, err := extractBinary(buf.Bytes(), "nope"); err == nil {
		t.Error("expected error when binary not present")
	}
}
