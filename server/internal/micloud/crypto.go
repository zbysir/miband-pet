package micloud

import (
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"sort"
	"strings"
	"time"
)

// 小米健康云接口的参数加密，移植自 huami-token 的 mi_crypto.py：
//   - RC4 密钥 = base64(SHA256(ssecurity || nonce))
//   - RC4 丢弃前 1024 字节
//   - 参数按 key 排序后用同一个 RC4 流依次加密（流是连续的，不是每个参数重新开始）
//   - 签名 = base64(SHA1("METHOD&path&k1=v1&...&rc4key"))

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func unb64(s string) []byte {
	b, _ := base64.StdEncoding.DecodeString(s)
	return b
}

// newNonce：8 字节随机数 + 当前分钟数（int32 大端）
func newNonce() string {
	b := make([]byte, 12)
	rand.Read(b[:8])
	binary.BigEndian.PutUint32(b[8:], uint32(time.Now().UnixMilli()/60000))
	return b64(b)
}

func rc4Key(ssecurity, nonce string) string {
	sum := sha256.Sum256(append(unb64(ssecurity), unb64(nonce)...))
	return b64(sum[:])
}

func newRC4(keyB64 string) *rc4.Cipher {
	c, _ := rc4.NewCipher(unb64(keyB64))
	drop := make([]byte, 1024)
	c.XORKeyStream(drop, drop)
	return c
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sha1Sign(method, path string, params map[string]string, key string) string {
	var parts []string
	if method != "" {
		parts = append(parts, strings.ToUpper(method))
	}
	if path != "" {
		parts = append(parts, path)
	}
	for _, k := range sortedKeys(params) {
		parts = append(parts, k+"="+params[k])
	}
	parts = append(parts, key)
	sum := sha1.Sum([]byte(strings.Join(parts, "&")))
	return b64(sum[:])
}

// encryptParams 返回要放进表单的参数（加密后的参数 + signature + _nonce）。
func encryptParams(method, path string, params map[string]string, nonce, ssecurity string) map[string]string {
	key := rc4Key(ssecurity, nonce)
	plain := map[string]string{}
	for k, v := range params {
		plain[k] = v
	}
	plain["rc4_hash__"] = sha1Sign(method, path, params, key)

	c := newRC4(key)
	enc := map[string]string{}
	for _, k := range sortedKeys(plain) {
		out := make([]byte, len(plain[k]))
		c.XORKeyStream(out, []byte(plain[k]))
		enc[k] = b64(out)
	}
	sig := sha1Sign(method, path, enc, key)
	enc["signature"] = sig
	enc["_nonce"] = nonce
	return enc
}

func decryptBody(body, nonce, ssecurity string) []byte {
	data := unb64(body)
	out := make([]byte, len(data))
	newRC4(rc4Key(ssecurity, nonce)).XORKeyStream(out, data)
	return out
}
