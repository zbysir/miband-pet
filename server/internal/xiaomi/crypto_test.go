package xiaomi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// testdata/vectors.json 由原来的 Python 实现（cryptography 库的 AESCCM 等）生成，用来保证两边结果一致
type vectors struct {
	CCM    []struct{ Key, Nonce, Msg, Out string }
	Derive []struct{ Secret, Phone, Watch, Out string }
	CTR    []struct{ Okm, Msg, Enc, Dec string }
	CRC16  []struct {
		Data string
		CRC  uint16
	}
}

func h(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func loadVectors(t *testing.T) vectors {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCCM(t *testing.T) {
	for _, c := range loadVectors(t).CCM {
		got := ccmSeal(h(c.Key), h(c.Nonce), h(c.Msg), 4)
		if !bytes.Equal(got, h(c.Out)) {
			t.Fatalf("ccmSeal(len %d) = %x, want %s", len(c.Msg)/2, got, c.Out)
		}
		plain, err := ccmOpen(h(c.Key), h(c.Nonce), got, 4)
		if err != nil || !bytes.Equal(plain, h(c.Msg)) {
			t.Fatalf("ccmOpen 往返失败: %v", err)
		}
		if len(got) > 0 {
			got[0] ^= 1
			if _, err := ccmOpen(h(c.Key), h(c.Nonce), got, 4); err == nil {
				t.Fatal("篡改后 ccmOpen 应该失败")
			}
		}
	}
}

func TestDeriveKeys(t *testing.T) {
	for _, c := range loadVectors(t).Derive {
		if got := deriveKeys(h(c.Secret), h(c.Phone), h(c.Watch)); !bytes.Equal(got, h(c.Out)) {
			t.Fatalf("deriveKeys = %x, want %s", got, c.Out)
		}
	}
}

func TestCTR(t *testing.T) {
	for _, c := range loadVectors(t).CTR {
		k := newKeys(h(c.Okm))
		if got := k.EncryptCTR(h(c.Msg)); !bytes.Equal(got, h(c.Enc)) {
			t.Fatalf("EncryptCTR = %x, want %s", got, c.Enc)
		}
		if got := k.DecryptCTR(h(c.Msg)); !bytes.Equal(got, h(c.Dec)) {
			t.Fatalf("DecryptCTR = %x, want %s", got, c.Dec)
		}
	}
}

func TestCRC16AndFrame(t *testing.T) {
	for _, c := range loadVectors(t).CRC16 {
		if got := crc16ARC(h(c.Data)); got != c.CRC {
			t.Fatalf("crc16 = %d, want %d", got, c.CRC)
		}
		enc := encodeFrame(frameData, 7, h(c.Data))
		// 前面塞点垃圾字节，确认能跳过
		f, n, ok := parseFrame(append([]byte{0x01, 0x02}, enc...))
		if ok || n != 2 {
			t.Fatalf("应先跳过 2 个垃圾字节，got n=%d ok=%v", n, ok)
		}
		f, n, ok = parseFrame(enc)
		if !ok || n != len(enc) || f.seq != 7 || !bytes.Equal(f.payload, h(c.Data)) {
			t.Fatalf("parseFrame 往返失败")
		}
		if len(enc) > 9 {
			if _, n, _ := parseFrame(enc[:len(enc)-1]); n != 0 {
				t.Fatal("不完整的帧应返回 n=0")
			}
		}
	}
}
