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
	"strings"
	"time"
)

const certPath = "META-INF/CERT"

// File 是安装包里的一个文件
type File struct {
	Name string // 包里的路径，如 pages/index/index.js
	Body []byte
}

// Build 用这些文件从零打一个签好名的安装包。文件按给的顺序放，所在目录的目录项自动补上；
// META-INF/CERT 由签名时生成。comment 是 zip 注释（aiot-toolkit 在这里写打包信息，手环不看）。
func Build(files []File, comment string, s *Signer) ([]byte, error) {
	now := time.Now()
	entries := []file{{name: "META-INF/", modified: now}, {name: certPath, modified: now}}
	dirs := map[string]bool{"META-INF/": true}
	for _, f := range files {
		parts := strings.Split(f.Name, "/")
		for i := 1; i < len(parts); i++ {
			d := strings.Join(parts[:i], "/") + "/"
			if !dirs[d] {
				dirs[d] = true
				entries = append(entries, file{name: d, modified: now})
			}
		}
		entries = append(entries, file{name: f.Name, body: f.Body, modified: now})
	}
	raw, err := writeZip(entries, comment)
	if err != nil {
		return nil, err
	}
	return Repack(raw, s, func(name string, b []byte) ([]byte, bool) { return b, true })
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
		// 和 aiot-toolkit 打的一样：版本 2.0、DOS 时间、目录带目录属性（CreateRaw 不会替我们填这些）
		mod := e.modified
		if mod.IsZero() {
			mod = time.Now()
		}
		h := &zip.FileHeader{Name: e.name, CreatorVersion: 20, ReaderVersion: 20}
		h.SetModTime(mod.UTC())
		if strings.HasSuffix(e.name, "/") {
			h.Method = zip.Store
			h.ReaderVersion = 10
			h.ExternalAttrs = 0x10
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
