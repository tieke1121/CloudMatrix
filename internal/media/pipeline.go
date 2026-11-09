package media

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// PipelineStats 媒体流实时运行指标
type PipelineStats struct {
	TotalPackets    uint64    `json:"total_packets"`
	DroppedPackets  uint64    `json:"dropped_packets"`
	KeyframePackets uint64    `json:"keyframe_packets"`
	LastFrameTime   time.Time `json:"last_frame_time"`
	CurrentFPS      float64   `json:"current_fps"`
}

// StreamPipeline 单个模拟器实例的媒体流处理管道
type StreamPipeline struct {
	mu           sync.RWMutex
	config       StreamConfig
	header       *StreamHeader
	outChan      chan *VideoPacket
	stats        PipelineStats
	ctx          context.Context
	cancel       context.CancelFunc
	closed       int32
	lastSentTime time.Time
	frameCounter uint64
	lastFPSCalc  time.Time
}

// NewStreamPipeline 创建新的媒体管道
func NewStreamPipeline(cfg StreamConfig) *StreamPipeline {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 30
	}

	return &StreamPipeline{
		config:      cfg,
		outChan:     make(chan *VideoPacket, cfg.BufferSize),
		lastFPSCalc: time.Now(),
	}
}

// Config 获取当前配置快照
func (p *StreamPipeline) Config() StreamConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig 动态调整画质与节流参数
func (p *StreamPipeline) UpdateConfig(cfg StreamConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
}

// Header 获取解析到的流初始头部信息
func (p *StreamPipeline) Header() *StreamHeader {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.header
}

// OutChannel 获取供客户端消费的输出数据通道
func (p *StreamPipeline) OutChannel() <-chan *VideoPacket {
	return p.outChan
}

// Stats 获取流运行指标统计
func (p *StreamPipeline) Stats() PipelineStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := p.stats
	return s
}

// Run 启动流读取与分发主循环，直到 context 取消或底层读取结束
func (p *StreamPipeline) Run(parentCtx context.Context, r io.Reader) error {
	p.mu.Lock()
	p.ctx, p.cancel = context.WithCancel(parentCtx)
	p.mu.Unlock()

	defer p.Close()

	// 1. 读取并校验 scrcpy 协议头部
	header, err := ParseHeader(r)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.header = header
	p.mu.Unlock()

	// 2. 循环读取视频帧
	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
		}

		packet, err := ReadPacket(r)
		if err != nil {
			return err
		}

		p.handlePacket(packet)
	}
}

// handlePacket 核心包处理：帧率节流、背压丢弃与状态采样
func (p *StreamPipeline) handlePacket(pkt *VideoPacket) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalPackets++
	if pkt.IsKey {
		p.stats.KeyframePackets++
	}

	// 1. 处于完全暂停状态时直接丢弃全部帧
	if p.config.Profile == QualityPaused || p.config.TargetFPS <= 0 {
		p.stats.DroppedPackets++
		return
	}

	// 2. 帧率节流限制 (非关键帧若发送过密则主动丢弃)
	now := time.Now()
	minInterval := time.Second / time.Duration(p.config.TargetFPS)
	if !pkt.IsKey && now.Sub(p.lastSentTime) < minInterval {
		p.stats.DroppedPackets++
		return
	}

	// 3. 背压保护：队列已满时优先保关键帧
	select {
	case p.outChan <- pkt:
		p.lastSentTime = now
		p.stats.LastFrameTime = now
		p.frameCounter++
	default:
		// 通道已满发生背压
		if pkt.IsKey {
			// 关键帧优先：尝试挤出一条旧帧
			select {
			case <-p.outChan:
				p.stats.DroppedPackets++
			default:
			}
			// 再次尝试推入关键帧
			select {
			case p.outChan <- pkt:
				p.lastSentTime = now
				p.stats.LastFrameTime = now
				p.frameCounter++
			default:
				p.stats.DroppedPackets++
			}
		} else {
			// 非关键帧直接丢弃，保障低延迟不积压
			p.stats.DroppedPackets++
		}
	}

	// 4. 定期计算估算 FPS (每 1 秒刷新一次)
	if now.Sub(p.lastFPSCalc) >= time.Second {
		elapsed := now.Sub(p.lastFPSCalc).Seconds()
		if elapsed > 0 {
			p.stats.CurrentFPS = float64(p.frameCounter) / elapsed
		}
		p.frameCounter = 0
		p.lastFPSCalc = now
	}
}

// Close 关闭媒体处理管道，释放缓冲区资源
func (p *StreamPipeline) Close() {
	if atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		p.mu.Lock()
		if p.cancel != nil {
			p.cancel()
		}
		close(p.outChan)
		p.mu.Unlock()
	}
}
