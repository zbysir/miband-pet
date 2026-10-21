package xiaomi

import "encoding/binary"

// SPP v2 帧：a5 a5 | 类型 | 序号 | 长度(u16 LE) | CRC16/ARC(u16 LE) | 负载
// 移植自 Gadgetbridge XiaomiSppPacketV2。手环 10 系列不管走经典蓝牙还是 BLE 都用这套帧。

var preamble = []byte{0xa5, 0xa5}

const (
	frameNAK           = 0
	frameACK           = 1
	frameSessionConfig = 2
	frameData          = 3
)

// 数据帧负载：通道 | opcode | 内容
const (
	chProtobuf = 1 // protobuf 命令，鉴权后加密
	chMass     = 2 // 大文件分片（装 rpk），明文
	opPlain    = 1
	opEncrypt  = 2
)

// 抓包自官方 App 的 start session：版本 1.0.0，最大帧 0xfc00，窗口 32，超时 10000ms
var sessionStart = []byte{
	1,
	1, 0x03, 0x00, 0x01, 0x00, 0x00,
	2, 0x02, 0x00, 0x00, 0xfc,
	3, 0x02, 0x00, 0x20, 0x00,
	4, 0x02, 0x00, 0x10, 0x27,
}

func crc16ARC(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func encodeFrame(kind, seq byte, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	copy(out, preamble)
	out[2] = kind & 0xF
	out[3] = seq
	binary.LittleEndian.PutUint16(out[4:], uint16(len(payload)))
	binary.LittleEndian.PutUint16(out[6:], crc16ARC(payload))
	copy(out[8:], payload)
	return out
}

type frame struct {
	kind, seq byte
	payload   []byte
}

// parseFrame 从缓冲区头部解析一帧。返回消耗的字节数；0 表示数据还不完整。
// 校验失败或不认识的字节会被跳过（返回 >0 且 ok=false）。
func parseFrame(buf []byte) (f frame, n int, ok bool) {
	if len(buf) < 2 {
		return f, 0, false
	}
	if buf[0] != preamble[0] || buf[1] != preamble[1] {
		for i := 1; i < len(buf); i++ {
			if buf[i] == preamble[0] {
				return f, i, false
			}
		}
		return f, len(buf), false
	}
	if len(buf) < 8 {
		return f, 0, false
	}
	length := int(binary.LittleEndian.Uint16(buf[4:]))
	total := 8 + length
	if len(buf) < total {
		return f, 0, false
	}
	payload := buf[8:total]
	if crc16ARC(payload) != binary.LittleEndian.Uint16(buf[6:]) {
		return f, total, false
	}
	return frame{kind: buf[2] & 0xF, seq: buf[3], payload: append([]byte{}, payload...)}, total, true
}
