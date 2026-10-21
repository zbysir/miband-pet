package xiaomi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
)

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// deriveKeys 即 Gadgetbridge XiaomiAuthService.computeAuthStep3Hmac：本质是 HKDF-SHA256，info="miwear-auth"，输出 64 字节。
func deriveKeys(secret, phoneNonce, watchNonce []byte) []byte {
	prk := hmacSHA256(append(append([]byte{}, phoneNonce...), watchNonce...), secret)
	var out, t []byte
	for i := byte(1); len(out) < 64; i++ {
		t = hmacSHA256(prk, append(append(t, "miwear-auth"...), i))
		out = append(out, t...)
	}
	return out[:64]
}

// Keys 是鉴权时派生出的会话密钥。
type Keys struct {
	DecKey, EncKey     []byte
	DecNonce, EncNonce []byte
}

func newKeys(okm []byte) *Keys {
	return &Keys{DecKey: okm[0:16], EncKey: okm[16:32], DecNonce: okm[32:36], EncNonce: okm[36:40]}
}

// EncryptCCM 用于鉴权第三步的设备信息（AES-CCM，4 字节 tag，nonce = encNonce | 0 | index）。
func (k *Keys) EncryptCCM(data []byte, index uint32) []byte {
	nonce := make([]byte, 12)
	copy(nonce, k.EncNonce)
	binary.LittleEndian.PutUint32(nonce[8:], index)
	return ccmSeal(k.EncKey, nonce, data, 4)
}

// SPP v2 的加密：AES-CTR，IV 直接用 key（Gadgetbridge 原注释：“I wish I was kidding”）
func (k *Keys) EncryptCTR(data []byte) []byte { return ctr(k.EncKey, data) }
func (k *Keys) DecryptCTR(data []byte) []byte { return ctr(k.DecKey, data) }

func ctr(key, data []byte) []byte {
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(data))
	cipher.NewCTR(block, key).XORKeyStream(out, data)
	return out
}

// ccmSeal 实现 RFC 3610 的 AES-CCM（无附加数据），nonce 12 字节，长度字段 3 字节。标准库没有 CCM。
func ccmSeal(key, nonce, plaintext []byte, tagLen int) []byte {
	block, _ := aes.NewCipher(key)
	const L = 3 // 15 - len(nonce)
	var b0 [16]byte
	b0[0] = byte(((tagLen-2)/2)<<3 | (L - 1))
	copy(b0[1:13], nonce)
	n := len(plaintext)
	b0[13], b0[14], b0[15] = byte(n>>16), byte(n>>8), byte(n)

	// CBC-MAC
	var mac [16]byte
	block.Encrypt(mac[:], b0[:])
	for i := 0; i < n; i += 16 {
		var blk [16]byte
		copy(blk[:], plaintext[i:min(i+16, n)])
		subtle.XORBytes(mac[:], mac[:], blk[:])
		block.Encrypt(mac[:], mac[:])
	}

	// CTR：A_i = flags(L-1) | nonce | i
	counter := func(i int) []byte {
		var a [16]byte
		a[0] = L - 1
		copy(a[1:13], nonce)
		a[13], a[14], a[15] = byte(i>>16), byte(i>>8), byte(i)
		var s [16]byte
		block.Encrypt(s[:], a[:])
		return s[:]
	}
	out := make([]byte, n+tagLen)
	for i := 0; i < n; i += 16 {
		end := min(i+16, n)
		subtle.XORBytes(out[i:end], plaintext[i:end], counter(i/16+1))
	}
	subtle.XORBytes(out[n:], mac[:tagLen], counter(0))
	return out
}

func ccmOpen(key, nonce, sealed []byte, tagLen int) ([]byte, error) {
	if len(sealed) < tagLen {
		return nil, errors.New("ccm: 密文太短")
	}
	n := len(sealed) - tagLen
	block, _ := aes.NewCipher(key)
	plain := make([]byte, n)
	for i := 0; i < n; i += 16 {
		end := min(i+16, n)
		var a [16]byte
		a[0] = 2
		copy(a[1:13], nonce)
		c := i/16 + 1
		a[13], a[14], a[15] = byte(c>>16), byte(c>>8), byte(c)
		var s [16]byte
		block.Encrypt(s[:], a[:])
		subtle.XORBytes(plain[i:end], sealed[i:end], s[:])
	}
	if subtle.ConstantTimeCompare(ccmSeal(key, nonce, plain, tagLen)[n:], sealed[n:]) != 1 {
		return nil, errors.New("ccm: tag 校验失败")
	}
	return plain, nil
}
