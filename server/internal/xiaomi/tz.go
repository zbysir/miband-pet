package xiaomi

import (
	"os"
	"strings"
)

// localZoneName 取 IANA 时区名（如 Asia/Shanghai），手环要这个格式。
func localZoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if p, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(p, "zoneinfo/"); i >= 0 {
			return p[i+len("zoneinfo/"):]
		}
	}
	return "UTC"
}
