package transport

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"cloudmatrix/api"
)

var (
	// ErrFrameTooLarge 帧体积超出上限
	ErrFrameTooLarge = errors.New("rpc frame exceeds maximum allowed size")
	// ErrVersionMismatch 协议版本不匹配
	ErrVersionMismatch = errors.New("rpc protocol version mismatch")
	// ErrUnauthorized 鉴权失败，Token 非法
	ErrUnauthorized = errors.New("unauthorized rpc request")
)

// MaxFrameSize 允许的最大 RPC 帧大小 (4 MiB)
const MaxFrameSize = 4 * 1024 * 1024

// WriteFrame 将 RPC 消息序列化并写入输出流 (4 字节长度前缀 + JSON 载荷)
func WriteFrame(w io.Writer, msg *api.RPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal rpc message failed: %w", err)
	}

	if len(data) > MaxFrameSize {
		return ErrFrameTooLarge
	}

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))

	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

// ReadFrame 从输入流中读取单个 RPC 帧并反序列化为消息对象
func ReadFrame(r io.Reader) (*api.RPCMessage, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}

	frameLen := binary.BigEndian.Uint32(lenBuf)
	if frameLen == 0 {
		return nil, errors.New("empty rpc frame")
	}
	if frameLen > MaxFrameSize {
		return nil, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, frameLen, MaxFrameSize)
	}

	payload := make([]byte, frameLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	var msg api.RPCMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal rpc frame failed: %w", err)
	}

	return &msg, nil
}
