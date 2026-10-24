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
	key, err := os.ReadFile("../../../spike/render-app/sign/debug/private.pem")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := os.ReadFile("../../../spike/render-app/sign/debug/certificate.pem")
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


// 从零打的包：文件都在、目录项补上了、META-INF/CERT 列了每个文件的摘要，签名能通过自己的校验
func TestBuild(t *testing.T) {
	s := testSigner(t)
	data, err := Build([]File{
		{Name: "manifest.json", Body: []byte(`{"package":"a.b"}`)},
		{Name: "app.js", Body: []byte("x")},
		{Name: "pages/index/index.js", Body: []byte("y")},
	}, "", s)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	want := "META-INF/ META-INF/CERT manifest.json app.js pages/ pages/index/ pages/index/index.js"
	if strings.Join(names, " ") != want {
		t.Fatalf("文件顺序 %v", names)
	}
	cert, err := readFile(data, certPath)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := zip.NewReader(bytes.NewReader(cert), int64(len(cert)))
	if err != nil {
		t.Fatalf("CERT 不是 zip：%v", err)
	}
	rc, _ := inner.File[0].Open()
	hash, _ := io.ReadAll(rc)
	if !strings.Contains(string(hash), `"pages/index/index.js"`) {
		t.Fatalf("hash.json 没列页面：%s", hash)
	}
	// 再签一遍应该不变（签名是确定的、和已有签名无关）
	again, err := Repack(data, s, func(name string, b []byte) ([]byte, bool) { return b, true })
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, data) {
		t.Fatal("重签后不一样")
	}
}
