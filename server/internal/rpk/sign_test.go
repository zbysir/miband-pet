package rpk

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"strings"
	"testing"
)

func testSigner(t *testing.T) *Signer {
	t.Helper()
	key, err := os.ReadFile("../../../pet-app/sign/debug/private.pem")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := os.ReadFile("../../../pet-app/sign/debug/certificate.pem")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSigner(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// 去掉 aiot-toolkit 打出来的签名，再用 Go 签一遍，结果要逐字节一致
func TestSignMatchesToolkit(t *testing.T) {
	s := testSigner(t)
	signed, err := os.ReadFile("testdata/pet.rpk")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := unsign(signed)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var files []FileHash
	var cert []byte
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		files = append(files, FileHash{Name: f.Name, Hash: sha256.Sum256(b)})
		if f.Name == "META-INF/CERT" {
			cert = b
		}
	}
	got, err := s.Sign(raw, files)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, signed) {
		t.Fatalf("整包签名和 aiot-toolkit 不一致：%d vs %d 字节", len(got), len(signed))
	}

	// META-INF/CERT 自己的签名：文件列表只有一项，名字是 hash.json，摘要是整个 CERT zip 的
	rawCert, err := unsign(cert)
	if err != nil {
		t.Fatal(err)
	}
	gotCert, err := s.Sign(rawCert, []FileHash{{Name: "hash.json", Hash: sha256.Sum256(rawCert)}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotCert, cert) {
		t.Fatalf("CERT 签名和 aiot-toolkit 不一致：%d vs %d 字节", len(gotCert), len(cert))
	}
}

func zipFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = b
	}
	return out
}

// 原样重新打包：hash.json 要和 aiot-toolkit 生成的逐字节一致；签名要能自洽（去掉再签一遍不变）
func TestRepackKeepsHashJSON(t *testing.T) {
	s := testSigner(t)
	orig, _ := os.ReadFile("testdata/pet.rpk")
	got, err := Repack(orig, s, func(name string, b []byte) ([]byte, bool) { return b, true })
	if err != nil {
		t.Fatal(err)
	}
	certOf := func(rpk []byte) []byte {
		c, _ := unsign(zipFiles(t, rpk)["META-INF/CERT"])
		return zipFiles(t, c)["hash.json"]
	}
	if !bytes.Equal(certOf(got), certOf(orig)) {
		t.Fatalf("hash.json 不一致:\n%s\n%s", certOf(got)[:200], certOf(orig)[:200])
	}
	checkResign(t, s, got)
}

func checkResign(t *testing.T, s *Signer, signed []byte) {
	t.Helper()
	raw, err := unsign(signed)
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	var files []FileHash
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files = append(files, FileHash{Name: f.Name, Hash: sha256.Sum256(b)})
	}
	again, err := s.Sign(raw, files)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, signed) {
		t.Fatal("签名不自洽")
	}
}

func TestForSkin(t *testing.T) {
	s := testSigner(t)
	orig, _ := os.ReadFile("testdata/pet.rpk")
	got, err := ForSkin(orig, "gray", s)
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, got)
	for name := range files {
		if strings.Contains(name, "/orange/") || strings.Contains(name, "/white/") {
			t.Fatalf("还留着别的猫: %s", name)
		}
	}
	page := string(files["pages/index/index.js"])
	if !petDir.MatchString(string(zipFiles(t, orig)["pages/index/index.js"])) || strings.Contains(page, "/orange/") || !strings.Contains(page, "/gray/") {
		t.Fatal("页面里的帧路径没换成灰猫")
	}
	if len(got) >= len(orig) {
		t.Fatalf("包没变小: %d >= %d", len(got), len(orig))
	}
	checkResign(t, s, got)
	if _, err := ForSkin(orig, "tiger", s); err == nil {
		t.Fatal("不存在的猫应该报错")
	}
	t.Logf("完整包 %d 字节，只带灰猫 %d 字节", len(orig), len(got))
}
