package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	// ErrPipelineAlreadyExists 对应实例的媒体管道已存在
	ErrPipelineAlreadyExists = errors.New("pipeline already exists for instance")
	// ErrPipelineNotFound 未找到对应实例的媒体管道
	ErrPipelineNotFound = errors.New("pipeline not found")
)

// VideoWallManager 管理多路视频墙流会话生命周期与自适应负载策略
type VideoWallManager struct {
	mu        sync.RWMutex
	pipelines map[string]*StreamPipeline
}

// NewVideoWallManager 创建视频墙管理器实例
func NewVideoWallManager() *VideoWallManager {
	return &VideoWallManager{
		pipelines: make(map[string]*StreamPipeline),
	}
}

// RegisterStream 注册并启动单个实例的媒体流
func (m *VideoWallManager) RegisterStream(ctx context.Context, instanceID string, r io.Reader, profile QualityProfile) (*StreamPipeline, error) {
	m.mu.Lock()
	if _, exists := m.pipelines[instanceID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrPipelineAlreadyExists, instanceID)
	}

	cfg := DefaultStreamConfig(instanceID, profile)
	pipeline := NewStreamPipeline(cfg)
	m.pipelines[instanceID] = pipeline
	m.mu.Unlock()

	// 触发全局画质自适应重平衡
	m.RebalanceQuality()

	// 在独立非 UI goroutine 中启动读取循环
	go func() {
		_ = pipeline.Run(ctx, r)
		m.UnregisterStream(instanceID)
	}()

	return pipeline, nil
}

// UnregisterStream 注销并关闭指定实例的媒体流
func (m *VideoWallManager) UnregisterStream(instanceID string) {
	m.mu.Lock()
	pipeline, exists := m.pipelines[instanceID]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.pipelines, instanceID)
	m.mu.Unlock()

	pipeline.Close()
	m.RebalanceQuality()
}

// GetPipeline 获取指定实例的流管道
func (m *VideoWallManager) GetPipeline(instanceID string) (*StreamPipeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pipeline, exists := m.pipelines[instanceID]
	if !exists {
		return nil, ErrPipelineNotFound
	}
	return pipeline, nil
}

// SetBackgroundThrottled 设置指定实例是否进入后台遮挡节流状态
func (m *VideoWallManager) SetBackgroundThrottled(instanceID string, throttled bool) error {
	m.mu.RLock()
	pipeline, exists := m.pipelines[instanceID]
	m.mu.RUnlock()

	if !exists {
		return ErrPipelineNotFound
	}

	cfg := pipeline.Config()
	if throttled {
		cfg.Profile = QualityThrottled
		cfg.TargetFPS = 3
		cfg.MaxBitrate = 300000
	} else {
		cfg.Profile = QualityWallPreview
		cfg.TargetFPS = 12
		cfg.MaxBitrate = 1200000
	}
	pipeline.UpdateConfig(cfg)
	return nil
}

// RebalanceQuality 根据当前活跃流数量自动平衡画质与帧率上限
func (m *VideoWallManager) RebalanceQuality() {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.pipelines)
	for _, p := range m.pipelines {
		cfg := p.Config()
		// 保留手动设置为后台节流或暂停的状态
		if cfg.Profile == QualityThrottled || cfg.Profile == QualityPaused {
			continue
		}

		if count <= 4 {
			cfg.TargetFPS = 15
			cfg.MaxWidth = 1280
			cfg.MaxHeight = 720
		} else if count <= 9 {
			cfg.TargetFPS = 10
			cfg.MaxWidth = 960
			cfg.MaxHeight = 540
		} else {
			cfg.TargetFPS = 5
			cfg.MaxWidth = 640
			cfg.MaxHeight = 360
		}
		p.UpdateConfig(cfg)
	}
}

// GetAllStats 获取所有正在运行流的实时指标统计
func (m *VideoWallManager) GetAllStats() map[string]PipelineStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]PipelineStats, len(m.pipelines))
	for id, p := range m.pipelines {
		result[id] = p.Stats()
	}
	return result
}

// ActiveCount 当前活跃流数量
func (m *VideoWallManager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pipelines)
}

// StopAll 关闭所有流并释放资源
func (m *VideoWallManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.pipelines {
		p.Close()
	}
	m.pipelines = make(map[string]*StreamPipeline)
}
