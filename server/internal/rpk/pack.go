package rpk

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	certPath = "META-INF/CERT"
	pagePath = "pages/index/index.js"
)

// 猫的帧在 common/pet/<哈希>/<皮肤>/ 下，页面里写的路径默认指向橘猫
var petDir = regexp.MustCompile(`/common/pet/([0-9a-f]{8})/orange/`)

// ForSkin 把 Agent 猫的完整安装包改成只带一只猫：删掉其它皮肤的帧，
// 页面里的帧路径换成这只猫的目录，然后重新签名。skin 为 orange 时也会去掉另外两只。
func ForSkin(data []byte, skin string, s *Signer) ([]byte, error) {
	var hash string
	page, err := readFile(data, pagePath)
	if err != nil {
		return nil, err
	}
	m := petDir.FindSubmatch(page)
	if m == nil {
		return nil, errors.New("快应用页面里没找到帧目录（common/pet/<哈希>/orange/）")
	}
	hash = string(m[1])
	keep := "common/pet/" + hash + "/" + skin + "/"
	if _, err := readFile(data, keep+"sleep_0.png"); err != nil {
		return nil, fmt.Errorf("安装包里没有 %s 这只猫", skin)
	}
	return Repack(data, s, func(name string, b []byte) ([]byte, bool) {
		if strings.HasPrefix(name, "common/pet/"+hash+"/") && name != "common/pet/"+hash+"/" && !strings.HasPrefix(name, keep) {
			return nil, false
		}
		if name == pagePath {
			return petDir.ReplaceAll(b, []byte("/common/pet/${1}/"+skin+"/")), true
		}
		return b, true
	})
}

// Repack 按 change 改写安装包里的文件（返回 false 表示删掉），重新生成 META-INF/CERT 并签名。
// 目录项（以 / 结尾）也会经过 change，内容为空。文件顺序和 zip 注释保持不变。
func Repack(data []byte, s *Signer, change func(name string, b []byte) ([]byte, bool)) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var entries []file
	for _, f := range zr.File {
		var b []byte
		if !strings.HasSuffix(f.Name, "/") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			b, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
		if f.Name != certPath {
			var ok bool
			if b, ok = change(f.Name, b); !ok {
				continue
			}
		}
		entries = append(entries, file{name: f.Name, body: b, modified: f.Modified})
	}

	// META-INF/CERT：只装 hash.json（除它以外每个文件的 SHA-256）的小 zip，自己也要签名
	var digests []string
	for _, e := range entries {
		if e.name == certPath || strings.HasSuffix(e.name, "/") {
			continue
		}
		sum := sha256.Sum256(e.body)
		name, _ := json.Marshal(e.name)
		digests = append(digests, fmt.Sprintf("%s:%q", name, hex.EncodeToString(sum[:])))
	}
	hashJSON := []byte(`{"algorithm":"SHA-256","digests":{` + strings.Join(digests, ",") + `}}`)
	certZip, err := writeZip([]file{{name: "hash.json", body: hashJSON}}, "")
	if err != nil {
		return nil, err
	}
	certSum := sha256.Sum256(certZip)
	cert, err := s.Sign(certZip, []FileHash{{Name: "hash.json", Hash: certSum}})
	if err != nil {
		return nil, err
	}

	var files []FileHash
	for i, e := range entries {
		if e.name == certPath {
			entries[i].body = cert
		}
		if !strings.HasSuffix(e.name, "/") {
			files = append(files, FileHash{Name: e.name, Hash: sha256.Sum256(entries[i].body)})
		}
	}
	if len(files) == 0 || files[0].Name != certPath {
		return nil, errors.New("安装包里没有 META-INF/CERT")
	}
	raw, err := writeZip(entries, zr.Comment)
	if err != nil {
		return nil, err
	}
	return s.Sign(raw, files)
}

type file struct {
	name     string
	body     []byte
	modified time.Time
}

// writeZip 写 zip：文件用 deflate 压缩，大小和 CRC 写在文件头里（不用数据描述符）。
func writeZip(entries []file, comment string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Modified: e.modified}
		if strings.HasSuffix(e.name, "/") {
			h.Method = zip.Store
			if _, err := zw.CreateRaw(h); err != nil {
				return nil, err
			}
			continue
		}
		var comp bytes.Buffer
		fw, _ := flate.NewWriter(&comp, flate.BestCompression)
		fw.Write(e.body)
		fw.Close()
		h.Method = zip.Deflate
		h.CRC32 = crc32.ChecksumIEEE(e.body)
		h.CompressedSize64 = uint64(comp.Len())
		h.UncompressedSize64 = uint64(len(e.body))
		w, err := zw.CreateRaw(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(comp.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := zw.SetComment(comment); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readFile(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	f, err := zr.Open(name)
	if err != nil {
		return nil, fmt.Errorf("安装包里没有 %s", name)
	}
	defer f.Close()
	return io.ReadAll(f)
}
