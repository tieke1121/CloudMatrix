package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"testing"
	"time"
)

// buildMockHeader 辅助函数：构造测试用的 scrcpy 协议头部字节
func buildMockHeader(deviceName string, codec string, width, height uint32) []byte {
	buf := make([]byte, HeaderLen)
	copy(buf[:64], []byte(deviceName))
	copy(buf[64:68], []byte(codec))
	binary.BigEndian.PutUint32(buf[68:72], width)
	binary.BigEndian.PutUint32(buf[72:76], height)
	return buf
}

// buildMockPacket 辅助函数：构造测试用的视频帧包字节
func buildMockPacket(pts uint64, payload []byte) []byte {
	buf := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint64(buf[:8], pts)
	binary.BigEndian.PutUint32(buf[8:12], uint32(len(payload)))
	copy(buf[12:], payload)
	return buf
}

// TestParseHeaderValidAndInvalid 验证 scrcpy 协议头部解析与防御
func TestParseHeaderValidAndInvalid(t *testing.T) {
	// 1. 合法头部
	validData := buildMockHeader("device_emulator_01", "h264", 1920, 1080)
	header, err := ParseHeader(bytes.NewReader(validData))
	if err != nil {
		t.Fatalf("ParseHeader failed: %v", err)
	}
	if header.DeviceName != "device_emulator_01" || header.CodecID != "h264" || header.Width != 1920 || header.Height != 1080 {
		t.Fatalf("unexpected header parsed: %+v", header)
	}

	// 2. 非法零尺寸头部
	invalidZero := buildMockHeader("bad_dev", "h264", 0, 1080)
	_, err = ParseHeader(bytes.NewReader(invalidZero))
	if err == nil {
		t.Fatal("expected error for 0 width, got nil")
	}

	// 3. 不完整截断头部
	_, err = ParseHeader(bytes.NewReader(validData[:30]))
	if err == nil {
		t.Fatal("expected error for truncated header, got nil")
	}
}

// TestReadPacketAndKeyframeDetection 验证视频包读取与关键帧探测
func TestReadPacketAndKeyframeDetection(t *testing.T) {
	// 构造包含 Annex-B 起始码的 IDR 关键帧 (naluType = 5)
	idrPayload := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}
	packetBytes := buildMockPacket(1000, idrPayload)

	pkt, err := ReadPacket(bytes.NewReader(packetBytes))
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if !pkt.IsKey {
		t.Fatal("expected IsKey == true for IDR frame")
	}
	if pkt.PTS != 1000 || len(pkt.Payload) != len(idrPayload) {
		t.Fatalf("unexpected packet metadata: %+v", pkt)
	}

	// 构造普通 P 帧 (naluType = 1)
	pPayload := []byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x01, 0x02}
	pBytes := buildMockPacket(1033, pPayload)
	pktP, err := ReadPacket(bytes.NewReader(pBytes))
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if pktP.IsKey {
		t.Fatal("expected IsKey == false for P frame")
	}

	// 构造超限异常大包
	oversizedMeta := make([]byte, 12)
	binary.BigEndian.PutUint64(oversizedMeta[:8], 2000)
	binary.BigEndian.PutUint32(oversizedMeta[8:12], MaxPacketSize+1024)
	_, err = ReadPacket(bytes.NewReader(oversizedMeta))
	if err == nil {
		t.Fatal("expected ErrPacketTooLarge, got nil")
	}
}

// TestPipelineBackpressureDropping 验证管道背压丢弃与关键帧优先策略
func TestPipelineBackpressureDropping(t *testing.T) {
	cfg := StreamConfig{
		InstanceID: "inst_test",
		TargetFPS:  30,
		Profile:    QualityFull,
		BufferSize: 2,
	}

	pipeline := NewStreamPipeline(cfg)
	defer pipeline.Close()

	// 塞入2个关键帧填满通道
	keyPkt := &VideoPacket{PTS: 100, IsKey: true, Payload: []byte{1, 2}}
	pipeline.handlePacket(keyPkt)
	pipeline.handlePacket(keyPkt)

	// 通道已满，再塞入非关键帧：必须直接被丢弃
	nonKeyPkt := &VideoPacket{PTS: 150, IsKey: false, Payload: []byte{3, 4}}
	pipeline.handlePacket(nonKeyPkt)

	stats := pipeline.Stats()
	if stats.DroppedPackets == 0 {
		t.Fatalf("expected dropped packets on backpressure, got %d", stats.DroppedPackets)
	}

	// 消费一个包
	<-pipeline.OutChannel()

	// 验证配置更新
	cfg.Profile = QualityPaused
	pipeline.UpdateConfig(cfg)
	pipeline.handlePacket(keyPkt)
	stats2 := pipeline.Stats()
	if stats2.DroppedPackets <= stats.DroppedPackets {
		t.Fatal("expected dropped count to increase in paused state")
	}
}

// TestVideoWallManagerMultiStream 验证多路视频墙管理与自适应降频
func TestVideoWallManagerMultiStream(t *testing.T) {
	mgr := NewVideoWallManager()
	defer mgr.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 使用 pipe 模拟持续的长连接输入流
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	// 先写入头部
	headerData := buildMockHeader("vm-01", "h264", 1280, 720)
	go func() {
		_, _ = pw.Write(headerData)
	}()

	// 注册第 1 路流
	p1, err := mgr.RegisterStream(ctx, "vm-01", pr, QualityWallPreview)
	if err != nil {
		t.Fatalf("RegisterStream vm-01 failed: %v", err)
	}

	// 等待流读取并填充 header
	time.Sleep(30 * time.Millisecond)
	if mgr.ActiveCount() != 1 {
		t.Fatalf("expected active count 1, got %d", mgr.ActiveCount())
	}

	// 尝试重复注册相同实例：必须拒绝
	_, err = mgr.RegisterStream(ctx, "vm-01", pr, QualityWallPreview)
	if err == nil {
		t.Fatal("expected error on duplicate register, got nil")
	}

	// 批量模拟创建至 10 路长连接流以验证自适应降频规则
	pipes := make([]*io.PipeWriter, 0, 9)
	for i := 2; i <= 10; i++ {
		instID := string(rune('A' + i))
		rPipe, wPipe := io.Pipe()
		pipes = append(pipes, wPipe)
		go func(id string, w *io.PipeWriter) {
			_, _ = w.Write(buildMockHeader(id, "h264", 1280, 720))
		}(instID, wPipe)

		_, err := mgr.RegisterStream(ctx, instID, rPipe, QualityWallPreview)
		if err != nil {
			t.Fatalf("RegisterStream %s failed: %v", instID, err)
		}
	}

	// 10 路流时，TargetFPS 应该自适应重平衡至 5
	p1Config := p1.Config()
	if p1Config.TargetFPS != 5 {
		t.Fatalf("expected TargetFPS to rebalance to 5 for 10 streams, got %d", p1Config.TargetFPS)
	}

	// 测试后台节流切换
	err = mgr.SetBackgroundThrottled("vm-01", true)
	if err != nil {
		t.Fatalf("SetBackgroundThrottled failed: %v", err)
	}
	if p1.Config().Profile != QualityThrottled {
		t.Fatalf("expected profile throttled, got %s", p1.Config().Profile)
	}

	// 清理管道与流
	for _, w := range pipes {
		_ = w.Close()
	}
	mgr.StopAll()
	if mgr.ActiveCount() != 0 {
		t.Fatalf("expected active count 0 after StopAll, got %d", mgr.ActiveCount())
	}
}
