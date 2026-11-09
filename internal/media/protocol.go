package media

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	// ErrInvalidHeader 头部数据非法
	ErrInvalidHeader = errors.New("invalid scrcpy stream header")
	// ErrPacketTooLarge 数据包尺寸超出上限保护阈值
	ErrPacketTooLarge = errors.New("packet size exceeds limit")
	// ErrStreamClosed 流已关闭
	ErrStreamClosed = errors.New("stream closed")
)

// HeaderLen scrcpy 视频流初始化头部长度 (64B 设备名 + 4B Codec + 4B 宽 + 4B 高)
const HeaderLen = 76

// MaxPacketSize 单个视频数据包允许的最大字节数 (10 MiB)
const MaxPacketSize = 10 * 1024 * 1024

// StreamHeader scrcpy 视频流初始化元数据
type StreamHeader struct {
	DeviceName string
	CodecID    string
	Width      uint32
	Height     uint32
}

// VideoPacket 解包后的独立视频数据帧
type VideoPacket struct {
	PTS     uint64
	IsKey   bool
	Payload []byte
}

// ParseHeader 从输入流中解析 scrcpy 视频流初始头部
func ParseHeader(r io.Reader) (*StreamHeader, error) {
	buf := make([]byte, HeaderLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHeader, err)
	}

	rawName := string(buf[:64])
	deviceName := strings.TrimRight(rawName, "\x00")

	codecID := string(buf[64:68])
	width := binary.BigEndian.Uint32(buf[68:72])
	height := binary.BigEndian.Uint32(buf[72:76])

	if width == 0 || height == 0 || width > 8192 || height > 8192 {
		return nil, fmt.Errorf("%w: invalid dimensions %dx%d", ErrInvalidHeader, width, height)
	}

	return &StreamHeader{
		DeviceName: deviceName,
		CodecID:    codecID,
		Width:      width,
		Height:     height,
	}, nil
}

// ReadPacket 从输入流中读取单个视频帧包
func ReadPacket(r io.Reader) (*VideoPacket, error) {
	metaBuf := make([]byte, 12)
	if _, err := io.ReadFull(r, metaBuf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrStreamClosed
		}
		return nil, err
	}

	pts := binary.BigEndian.Uint64(metaBuf[:8])
	pktSize := binary.BigEndian.Uint32(metaBuf[8:12])

	if pktSize == 0 {
		return &VideoPacket{PTS: pts, IsKey: false, Payload: nil}, nil
	}

	if pktSize > MaxPacketSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrPacketTooLarge, pktSize, MaxPacketSize)
	}

	payload := make([]byte, pktSize)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("read payload failed: %w", err)
	}

	isKey := detectKeyframe(payload)

	return &VideoPacket{
		PTS:     pts,
		IsKey:   isKey,
		Payload: payload,
	}, nil
}

// detectKeyframe 检测 H.264/H.265 数据包中是否包含关键帧 (IDR/SPS/PPS)
func detectKeyframe(payload []byte) bool {
	length := len(payload)
	for i := 0; i < length-4; i++ {
		// 探测 00 00 01 或 00 00 00 01 Annex-B 起始码
		if payload[i] == 0 && payload[i+1] == 0 {
			naluStart := -1
			if payload[i+2] == 1 {
				naluStart = i + 3
			} else if payload[i+2] == 0 && payload[i+3] == 1 {
				naluStart = i + 4
			}

			if naluStart > 0 && naluStart < length {
				naluType := payload[naluStart] & 0x1F
				// 5 为 IDR 关键帧，7 为 SPS，8 为 PPS
				if naluType == 5 || naluType == 7 || naluType == 8 {
					return true
				}
			}
		}
	}
	return false
}
