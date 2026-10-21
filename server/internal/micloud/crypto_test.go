package micloud

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/vectors.json 由 huami-token 的 mi_crypto.py 在固定 ssecurity/nonce 下生成
func TestEncryptMatchesPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Ssecurity, Nonce, Rc4key, Body, Plain string
		SigningPath                           string `json:"signing_path"`
		Params, Encrypted                     map[string]string
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if got := rc4Key(v.Ssecurity, v.Nonce); got != v.Rc4key {
		t.Fatalf("rc4Key = %s, want %s", got, v.Rc4key)
	}
	got := encryptParams("POST", v.SigningPath, v.Params, v.Nonce, v.Ssecurity)
	for k, want := range v.Encrypted {
		if got[k] != want {
			t.Errorf("%s = %s, want %s", k, got[k], want)
		}
	}
	if len(got) != len(v.Encrypted) {
		t.Errorf("参数个数 %d, want %d", len(got), len(v.Encrypted))
	}
	if plain := decryptBody(v.Body, v.Nonce, v.Ssecurity); string(plain) != v.Plain {
		t.Errorf("decryptBody = %s", plain)
	}
}
