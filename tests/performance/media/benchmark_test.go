package media_perf_test

import (
	"context"
	"encoding/binary"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"cloudmatrix/internal/media"
)

// buildPerfHeader 构造性能测试用的协议头
func buildPerfHeader(name string, width, height uint32) []byte {
	buf := make([]byte, media.HeaderLen)
	copy(buf[:64], []byte(name))
	copy(buf[64:68], []byte("h264"))
	binary.BigEndian.PutUint32(buf[68:72], width)
	binary.BigEndian.PutUint32(buf[72:76], height)
	return buf
}

// buildPerfFrame 构造包含 Annex-B 起始码的性能测试帧
func buildPerfFrame(pts uint64, isKey bool, size int) []byte {
	buf := make([]byte, 12+size)
	binary.BigEndian.PutUint64(buf[:8], pts)
	binary.BigEndian.PutUint32(buf[8:12], uint32(size))
	// 填充起始码
	if size >= 5 {
		buf[12] = 0x00
		buf[13] = 0x00
		buf[14] = 0x00
		buf[15] = 0x01
		if isKey {
			buf[16] = 0x65
		} else {
			buf[16] = 0x41
		}
	}
	return buf
}

// TestMediaPipeline4WaysPerf 验证 4 路并发视频流性能与丢帧隔离
func TestMediaPipeline4WaysPerf(t *testing.T) {
	mgr := media.NewVideoWallManager()
	defer mgr.StopAll()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const streamsCount = 4
	const framesPerStream = 200

	var wg sync.WaitGroup
	wg.Add(streamsCount)

	startMem := runtime.MemStats{}
	runtime.ReadMemStats(&startMem)
	startTime := time.Now()

	pipelines := make([]*media.StreamPipeline, streamsCount)
	statsSnapshots := make([]media.PipelineStats, streamsCount)

	for i := 0; i < streamsCount; i++ {
		instID := "stream-node-" + string(rune('0'+i))
		pr, pw := io.Pipe()

		// 启动生产者 goroutine
		go func(id string, w *io.PipeWriter) {
			defer w.Close()
			header := buildPerfHeader(id, 1280, 720)
			_, _ = w.Write(header)

			for f := 0; f < framesPerStream; f++ {
				// 每 30 帧产生 1 个关键帧，帧体积 10KB
				isKey := (f % 30) == 0
				frame := buildPerfFrame(uint64(f*33), isKey, 10240)
				_, err := w.Write(frame)
				if err != nil {
					return
				}
				// 模拟 30fps 间隔 (略微加速以测试背压能力)
				time.Sleep(2 * time.Millisecond)
			}
		}(instID, pw)

		// 注册媒体流
		pipeline, err := mgr.RegisterStream(ctx, instID, pr, media.QualityWallPreview)
		if err != nil {
			t.Fatalf("RegisterStream %s failed: %v", instID, err)
		}
		pipelines[i] = pipeline

		// 启动模拟消费者
		idx := i
		go func(p *media.StreamPipeline) {
			defer wg.Done()
			consumed := 0
			for range p.OutChannel() {
				consumed++
				// 模拟前端消费处理耗时
				time.Sleep(3 * time.Millisecond)
			}
			statsSnapshots[idx] = p.Stats()
		}(pipeline)
	}

	wg.Wait()
	duration := time.Since(startTime)

	endMem := runtime.MemStats{}
	runtime.ReadMemStats(&endMem)

	var totalPackets, droppedPackets, keyframePackets uint64
	for _, s := range statsSnapshots {
		totalPackets += s.TotalPackets
		droppedPackets += s.DroppedPackets
		keyframePackets += s.KeyframePackets
	}

	t.Logf("4-way Media Wall Test finished in %v", duration)
	t.Logf("Total packets: %d, Keyframes: %d, Dropped on backpressure: %d (%.2f%%)",
		totalPackets, keyframePackets, droppedPackets, float64(droppedPackets)/float64(totalPackets)*100)
	t.Logf("Memory Alloc delta: %d KiB", (endMem.Alloc-startMem.Alloc)/1024)

	// 验证背压机制生效，且系统未崩溃未死锁
	if totalPackets == 0 {
		t.Fatal("expected total packets > 0")
	}
	if droppedPackets == 0 {
		t.Fatal("expected dropped packets > 0 under fast producer backpressure")
	}
}
