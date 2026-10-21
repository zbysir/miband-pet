// Package micloud 扫码登录小米账号，从小米健康云拉手环的 auth key（encrypt_key）和 MAC。
//
// 移植自 huami-token（https://github.com/argrento/huami-token）的 Xiaomi 部分，登录改用扫码：
// 账号密码登录会被小米的二次验证拦下（"Missing ssecurity or location"）。
package micloud

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	userAgent    = "Mozilla/5.0 (Linux; Android 12; Pixel 4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Mobile Safari/537.36"
	apiUserAgent = "Android-12-9.8.348i-google-Pixel 4"
	sid          = "miothealth"
)

// Session 是登录后的凭证，可以缓存下来，下次拉 key 不用再扫码。
type Session struct {
	Ssecurity    string `json:"ssecurity"`
	ServiceToken string `json:"service_token"`
	UserID       string `json:"user_id"`
	CUserID      string `json:"c_user_id"`
}

// Device 是账号下绑定的穿戴设备。
type Device struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	AuthKey string `json:"authKey"` // 0x 开头的 32 位十六进制；为空表示云端没有 key
	Model   string `json:"model"`
}

// parseJSON 去掉小米接口的 "&&&START&&&" 前缀再解析。
func parseJSON(b []byte, v any) error {
	s := strings.TrimPrefix(string(b), "&&&START&&&")
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("解析小米接口返回失败: %w: %.200s", err, s)
	}
	return nil
}

// QRLogin 是一次扫码登录。先 NewQRLogin 拿二维码，再 Wait 等用户扫码确认。
type QRLogin struct {
	QRImage []byte // 二维码图片（PNG）
	QRURL   string // 二维码图片地址
	client  *http.Client
	lp      string
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 30 * time.Second}
}

func get(ctx context.Context, c *http.Client, rawURL string, query url.Values) ([]byte, error) {
	if query != nil {
		rawURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// NewQRLogin 请求一个登录二维码。
func NewQRLogin(ctx context.Context) (*QRLogin, error) {
	c := newClient()
	// deviceId 像官方 App 一样带上；随便一个稳定值即可
	sum := md5.Sum([]byte(fmt.Sprint(time.Now().UnixNano())))
	u, _ := url.Parse("https://account.xiaomi.com")
	c.Jar.SetCookies(u, []*http.Cookie{{Name: "deviceId", Value: "an_" + hex.EncodeToString(sum[:])}})

	raw, err := get(ctx, c, "https://account.xiaomi.com/pass/serviceLogin", url.Values{"_json": {"true"}, "sid": {sid}, "_locale": {"zh_CN"}})
	if err != nil {
		return nil, err
	}
	var page struct {
		Sign         string `json:"_sign"`
		Qs           string `json:"qs"`
		Callback     string `json:"callback"`
		ServiceParam string `json:"serviceParam"`
	}
	if err := parseJSON(raw, &page); err != nil {
		return nil, err
	}

	raw, err = get(ctx, c, "https://account.xiaomi.com/longPolling/loginUrl", url.Values{
		"_qrsize": {"480"}, "qs": {page.Qs}, "bizDeviceType": {""}, "callback": {page.Callback},
		"_json": {"true"}, "theme": {""}, "sid": {sid}, "needTheme": {"false"}, "showActiveX": {"false"},
		"serviceParam": {page.ServiceParam}, "_locale": {"zh_CN"}, "_sign": {page.Sign},
		"_dc": {fmt.Sprint(time.Now().UnixMilli())},
	})
	if err != nil {
		return nil, err
	}
	var qr struct {
		QR string `json:"qr"`
		LP string `json:"lp"`
	}
	if err := parseJSON(raw, &qr); err != nil {
		return nil, err
	}
	if qr.QR == "" || qr.LP == "" {
		return nil, fmt.Errorf("拿二维码失败: %.200s", raw)
	}
	img, err := get(ctx, c, qr.QR, nil)
	if err != nil {
		return nil, err
	}
	return &QRLogin{QRImage: img, QRURL: qr.QR, client: c, lp: qr.LP}, nil
}

// Wait 长轮询等用户在手机上扫码并确认，成功后返回会话。ctx 控制总等待时间。
func (q *QRLogin) Wait(ctx context.Context) (*Session, error) {
	lp := &http.Client{Jar: q.client.Jar, Timeout: 70 * time.Second}
	for {
		if err := ctx.Err(); err != nil {
			return nil, errors.New("等扫码超时")
		}
		raw, err := get(ctx, lp, q.lp, nil)
		if err != nil {
			var ue *url.Error
			if errors.As(err, &ue) && ue.Timeout() {
				continue // 长轮询超时，接着等
			}
			return nil, err
		}
		var r struct {
			Ssecurity string `json:"ssecurity"`
			Location  string `json:"location"`
			Nonce     any    `json:"nonce"`
			UserID    any    `json:"userId"`
			CUserID   string `json:"cUserId"`
		}
		if err := parseJSON(raw, &r); err != nil || r.Ssecurity == "" {
			time.Sleep(time.Second)
			continue
		}
		s := &Session{Ssecurity: r.Ssecurity, UserID: fmt.Sprint(r.UserID), CUserID: r.CUserID}
		return s, q.serviceToken(ctx, s, r.Location, r.Nonce)
	}
}

// serviceToken 跟着 location 跳转拿 serviceToken cookie；带 nonce 时要附 clientSign。
func (q *QRLogin) serviceToken(ctx context.Context, s *Session, location string, nonce any) error {
	target := location
	if nonce != nil {
		n := fmt.Sprint(nonce)
		if f, ok := nonce.(float64); ok {
			n = fmt.Sprintf("%.0f", f)
		}
		sum := sha1.Sum([]byte("nonce=" + n + "&" + s.Ssecurity))
		target += "&clientSign=" + url.QueryEscape(b64(sum[:]))
	}
	if _, err := get(ctx, q.client, target, nil); err != nil {
		return err
	}
	for _, host := range []string{"https://sts-hlth.io.mi.com", "https://hlth.io.mi.com", location} {
		u, err := url.Parse(host)
		if err != nil {
			continue
		}
		for _, c := range q.client.Jar.Cookies(u) {
			if c.Name == "serviceToken" {
				s.ServiceToken = c.Value
				return nil
			}
		}
	}
	return errors.New("登录成功但没拿到 serviceToken")
}

// Devices 拉账号下绑定的穿戴设备（含 auth key）。
func (s *Session) Devices(ctx context.Context) ([]Device, error) {
	const endpoint = "https://hlth.io.mi.com/app/v1/source/get_source_list"
	data, _ := json.Marshal(map[string]int{"page_size": 50, "status": 1})
	nonce := newNonce()
	form := url.Values{}
	for k, v := range encryptParams("POST", "/app/v1/source/get_source_list", map[string]string{"data": string(data)}, nonce, s.Ssecurity) {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", apiUserAgent)
	req.Header.Set("region_tag", "ru")
	req.AddCookie(&http.Cookie{Name: "cUserId", Value: s.CUserID})
	req.AddCookie(&http.Cookie{Name: "serviceToken", Value: s.ServiceToken})
	req.AddCookie(&http.Cookie{Name: "locale", Value: "en_us"})
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("拉设备列表失败 (HTTP %d)，登录可能过期了", resp.StatusCode)
	}
	plain := decryptBody(string(body), nonce, s.Ssecurity)
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			List []struct {
				Name   string          `json:"name"`
				Model  string          `json:"model"`
				Detail json.RawMessage `json:"detail"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(plain, &r); err != nil {
		return nil, fmt.Errorf("拉设备列表失败，登录可能过期了: %w", err)
	}
	if r.Code != 0 {
		return nil, fmt.Errorf("拉设备列表失败: %d %s", r.Code, r.Message)
	}
	var out []Device
	for _, src := range r.Result.List {
		// detail 有时是 JSON 字符串，有时是对象
		detailRaw := src.Detail
		var str string
		if json.Unmarshal(detailRaw, &str) == nil {
			detailRaw = json.RawMessage(str)
		}
		var d struct {
			AuthKey    string `json:"auth_key"`
			EncryptKey string `json:"encrypt_key"`
			Token      string `json:"token"`
			MAC        string `json:"mac"`
		}
		json.Unmarshal(detailRaw, &d)
		key := d.AuthKey
		if key == "" {
			key = d.EncryptKey // 手环 10 系列的 key 在 encrypt_key（和 token 相同）
		}
		if key == "" {
			key = d.Token
		}
		if key != "" && !strings.HasPrefix(key, "0x") {
			key = "0x" + key
		}
		out = append(out, Device{Name: strings.TrimSpace(src.Name), MAC: strings.ToUpper(d.MAC), AuthKey: key, Model: src.Model})
	}
	return out, nil
}
