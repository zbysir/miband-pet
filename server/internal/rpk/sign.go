// Package rpk 读写手环快应用安装包（.rpk）。
//
// rpk 是 zip，签名方式照搬 aiot-toolkit（@aiot-toolkit/aiotpack 的 signature/SignUtil.js），
// 和安卓 APK v2 签名类似：
//
//  1. 算 zip 三段（文件数据、中央目录、结尾记录）的摘要，拼成整体摘要；
//  2. 用 RSA-SHA256 签「整体摘要 + 证书」，再签「每个文件名的 CRC32 + 文件 SHA-256」列表；
//  3. 把签名块（以 "RPK Sig Block 42" 结尾）插在中央目录前面，改结尾记录里中央目录的偏移。
//
// META-INF/CERT 是一个只装 hash.json 的小 zip，它自己也按同样方式签一次。
// RSA PKCS#1 v1.5 签名是确定的，同样的输入签出来和 aiot-toolkit 逐字节一致（见测试）。
package rpk

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	sigMagic     = "RPK Sig Block 42"
	idSignBlocks = 0x01000101 // 整体签名
	idFileHashes = 0x01000201 // 文件摘要签名
	algRSASHA256 = 0x0103
)

// FileHash 是签名里要列出的一个文件：zip 里的路径和内容的 SHA-256。
type FileHash struct {
	Name string
	Hash [32]byte
}

// Signer 持有签名用的私钥和证书。
type Signer struct {
	key     *rsa.PrivateKey
	certDER []byte
	pubDER  []byte
}

// NewSigner 从 PEM 格式的私钥（PKCS#8 或 PKCS#1）和证书创建 Signer。
func NewSigner(keyPEM, certPEM []byte) (*Signer, error) {
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, errors.New("私钥不是 PEM 格式")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS8PrivateKey(kb.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("私钥不是 RSA")
		}
		key = rk
	} else if rk, err2 := x509.ParsePKCS1PrivateKey(kb.Bytes); err2 == nil {
		key = rk
	} else {
		return nil, fmt.Errorf("解析私钥: %w", err)
	}
	cb, _ := pem.Decode(certPEM)
	if cb == nil {
		return nil, errors.New("证书不是 PEM 格式")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析证书: %w", err)
	}
	return &Signer{key: key, certDER: cb.Bytes, pubDER: cert.RawSubjectPublicKeyInfo}, nil
}

// Sign 给一个还没签名的 zip 加签名块，files 是要写进文件摘要签名的文件列表。
func (s *Signer) Sign(zipData []byte, files []FileHash) ([]byte, error) {
	sec, err := splitZip(zipData)
	if err != nil {
		return nil, err
	}
	// 1. 三段各自的摘要，再拼成整体摘要
	whole := []byte{0x5a}
	whole = binary.LittleEndian.AppendUint32(whole, 3)
	for _, part := range [][]byte{sec.header, sec.central, sec.eocd} {
		h := sha256.New()
		h.Write([]byte{0xa5})
		h.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(part))))
		h.Write(part)
		whole = h.Sum(whole)
	}
	digest := sha256.Sum256(whole)

	// 2. 签名块
	block, err := s.signBlock(digest[:])
	if err != nil {
		return nil, err
	}
	kvs := [][]byte{kv(idSignBlocks, block)}
	if files != nil {
		fh, err := s.fileHashes(files)
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, kv(idFileHashes, fh))
	}
	size := 24
	for _, k := range kvs {
		size += len(k)
	}
	var chunk []byte
	chunk = binary.LittleEndian.AppendUint32(chunk, uint32(size))
	chunk = binary.LittleEndian.AppendUint32(chunk, 0)
	for _, k := range kvs {
		chunk = append(chunk, k...)
	}
	chunk = binary.LittleEndian.AppendUint32(chunk, uint32(size))
	chunk = binary.LittleEndian.AppendUint32(chunk, 0)
	chunk = append(chunk, sigMagic...)

	// 3. 插到中央目录前面，改结尾记录里中央目录的偏移
	eocd := bytes.Clone(sec.eocd)
	binary.LittleEndian.PutUint32(eocd[16:], uint32(len(sec.header)+len(chunk)))
	out := make([]byte, 0, len(zipData)+len(chunk))
	out = append(out, sec.header...)
	out = append(out, chunk...)
	out = append(out, sec.central...)
	out = append(out, eocd...)
	return out, nil
}

// signBlock：签名数据（整体摘要 + 证书）、对它的签名、公钥。
func (s *Signer) signBlock(digest []byte) ([]byte, error) {
	var digests []byte
	digests = binary.LittleEndian.AppendUint32(digests, uint32(len(digest)+8))
	digests = binary.LittleEndian.AppendUint32(digests, algRSASHA256)
	digests = binary.LittleEndian.AppendUint32(digests, uint32(len(digest)))
	digests = append(digests, digest...)
	var certs []byte
	certs = binary.LittleEndian.AppendUint32(certs, uint32(len(s.certDER)))
	certs = append(certs, s.certDER...)

	var data []byte
	data = lenPrefixed(data, digests)
	data = lenPrefixed(data, certs)
	data = binary.LittleEndian.AppendUint32(data, 0) // additional attributes

	sig, err := s.sign(data)
	if err != nil {
		return nil, err
	}
	var sigs []byte
	sigs = binary.LittleEndian.AppendUint32(sigs, uint32(len(sig)+8))
	sigs = binary.LittleEndian.AppendUint32(sigs, algRSASHA256)
	sigs = binary.LittleEndian.AppendUint32(sigs, uint32(len(sig)))
	sigs = append(sigs, sig...)

	var b []byte
	b = lenPrefixed(b, data)
	b = lenPrefixed(b, sigs)
	b = lenPrefixed(b, s.pubDER)
	// 外面再包一层：签名块列表（这里只有一个）
	return lenPrefixed(nil, lenPrefixed(nil, b)), nil
}

// fileHashes：每个文件名的 CRC32 + 内容 SHA-256，整体签一次。
func (s *Signer) fileHashes(files []FileHash) ([]byte, error) {
	list := binary.LittleEndian.AppendUint32(nil, algRSASHA256)
	for _, f := range files {
		list = binary.LittleEndian.AppendUint32(list, crc32.ChecksumIEEE([]byte(f.Name)))
		list = binary.LittleEndian.AppendUint16(list, uint16(len(f.Hash)))
		list = append(list, f.Hash[:]...)
	}
	sig, err := s.sign(list)
	if err != nil {
		return nil, err
	}
	var b []byte
	b = lenPrefixed(b, list)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(sig)+8))
	b = binary.LittleEndian.AppendUint32(b, algRSASHA256)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(sig)))
	b = append(b, sig...)
	return lenPrefixed(nil, b), nil
}

func (s *Signer) sign(data []byte) ([]byte, error) {
	h := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, h[:])
}

// kv：长度（8 字节）+ id + 值
func kv(id uint32, value []byte) []byte {
	var b []byte
	b = binary.LittleEndian.AppendUint32(b, uint32(len(value)+4))
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = binary.LittleEndian.AppendUint32(b, id)
	return append(b, value...)
}

func lenPrefixed(dst, b []byte) []byte {
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(b)))
	return append(dst, b...)
}

type zipSections struct {
	header, central, eocd []byte
}

// splitZip 把 zip 切成文件数据、中央目录、结尾记录三段。
func splitZip(b []byte) (zipSections, error) {
	if len(b) < 22 || binary.LittleEndian.Uint32(b) != 0x04034b50 {
		return zipSections{}, errors.New("不是 zip")
	}
	end := -1
	for i := len(b) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(b[i:]) == 0x06054b50 {
			end = i
			break
		}
	}
	if end < 0 {
		return zipSections{}, errors.New("zip 没有结尾记录")
	}
	cd := int(binary.LittleEndian.Uint32(b[end+16:]))
	if cd > end || binary.LittleEndian.Uint32(b[cd:]) != 0x02014b50 {
		return zipSections{}, errors.New("zip 中央目录位置不对")
	}
	return zipSections{header: b[:cd], central: b[cd:end], eocd: b[end:]}, nil
}

// unsign 去掉签名块，还原成签名前的 zip（测试和重新打包用）。
func unsign(b []byte) ([]byte, error) {
	sec, err := splitZip(b)
	if err != nil {
		return nil, err
	}
	h := sec.header
	if !bytes.HasSuffix(h, []byte(sigMagic)) {
		return bytes.Clone(b), nil
	}
	// 签名块结尾：size(8 字节) + magic；size 不含开头的 8 字节
	size := int(binary.LittleEndian.Uint64(h[len(h)-len(sigMagic)-8:]))
	start := len(h) - size - 8
	if start < 0 {
		return nil, errors.New("签名块长度不对")
	}
	eocd := bytes.Clone(sec.eocd)
	binary.LittleEndian.PutUint32(eocd[16:], uint32(start))
	out := append(bytes.Clone(h[:start]), sec.central...)
	return append(out, eocd...), nil
}
