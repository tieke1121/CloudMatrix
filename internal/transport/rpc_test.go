package transport

import (
	"bytes"
	"encoding/binary"
	"testing"

	"cloudmatrix/api"
)

// TestWriteAndReadFrameValid 验证正常 RPC 帧的序列化与反序列化
func TestWriteAndReadFrameValid(t *testing.T) {
	var buf bytes.Buffer
	origMsg := &api.RPCMessage{
		Version:   api.RPCVersion,
		HostID:    "host-1",
		RequestID: "req-100",
		Token:     "tok-abc",
		Action:    api.ActionPing,
		Payload:   []byte(`{"msg":"hello"}`),
	}

	if err := WriteFrame(&buf, origMsg); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	readMsg, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if readMsg.Version != origMsg.Version || readMsg.RequestID != origMsg.RequestID || readMsg.Action != origMsg.Action {
		t.Fatalf("unexpected message parsed: %+v", readMsg)
	}
}

// TestReadFrameOversizedAndEmpty 验证畸形帧与超大帧拦截
func TestReadFrameOversizedAndEmpty(t *testing.T) {
	// 1. 空帧
	emptyBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(emptyBuf, 0)
	_, err := ReadFrame(bytes.NewReader(emptyBuf))
	if err == nil {
		t.Fatal("expected error for empty frame, got nil")
	}

	// 2. 超限大帧
	oversizedBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(oversizedBuf, MaxFrameSize+1024)
	_, err = ReadFrame(bytes.NewReader(oversizedBuf))
	if err == nil {
		t.Fatal("expected ErrFrameTooLarge, got nil")
	}
}
