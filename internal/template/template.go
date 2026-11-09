package template

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloudmatrix/internal/domain"
)

var (
	// ErrTemplateInUse 模板正被使用，拒绝删除
	ErrTemplateInUse = errors.New("TEMPLATE_IN_USE: 模板引用计数大于 0，禁止删除")
	// ErrInsufficientDisk 磁盘空间不足
	ErrInsufficientDisk = errors.New("INSUFFICIENT_DISK: 磁盘可用空间低于安全准入阈值")
)

// Template 模板领域模型
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Digest      string    `json:"digest"`
	RefCount    int       `json:"ref_count"`
	MinDiskGB   int       `json:"min_disk_gb"`
	IsImmutable bool      `json:"is_immutable"`
	CreatedAt   time.Time `json:"created_at"`
}

// Manager 模板生命周期管理接口
type Manager interface {
	CreateTemplate(ctx context.Context, name string, version string, digest string) (*Template, error)
	GetTemplate(id string) (*Template, error)
	ListTemplates() []*Template
	CloneInstance(ctx context.Context, templateID string, targetInstanceName string, diskFreeGB int) (*Template, error)
	ReleaseInstance(templateID string) error
	DeleteTemplate(templateID string) error
}

// MemoryTemplateManager 模板管理器实现
type MemoryTemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*Template
	minDiskGB int
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(minDiskGB int) *MemoryTemplateManager {
	if minDiskGB <= 0 {
		minDiskGB = 10
	}
	return &MemoryTemplateManager{
		templates: make(map[string]*Template),
		minDiskGB: minDiskGB,
	}
}

// CreateTemplate 创建新模板并冻结为不可变基底
func (m *MemoryTemplateManager) CreateTemplate(ctx context.Context, name string, version string, digest string) (*Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("tpl-%d", time.Now().UnixNano())
	tpl := &Template{
		ID:          id,
		Name:        name,
		Version:     version,
		Digest:      digest,
		RefCount:    0,
		MinDiskGB:   m.minDiskGB,
		IsImmutable: true,
		CreatedAt:   time.Now().UTC(),
	}

	m.templates[id] = tpl
	return tpl, nil
}

// GetTemplate 获取指定模板
func (m *MemoryTemplateManager) GetTemplate(id string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.templates[id]
	if !ok {
		return nil, domain.NewPlatformError(domain.ErrCodeInstanceNotFound, "指定模板不存在", false)
	}
	return tpl, nil
}

// ListTemplates 列出所有可用模板
func (m *MemoryTemplateManager) ListTemplates() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Template, 0, len(m.templates))
	for _, t := range m.templates {
		list = append(list, t)
	}
	return list
}

// CloneInstance 从模板克隆实例，严格校验磁盘空间并自增引用计数
func (m *MemoryTemplateManager) CloneInstance(ctx context.Context, templateID string, targetInstanceName string, diskFreeGB int) (*Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil, domain.NewPlatformError(domain.ErrCodeInstanceNotFound, "模板不存在", false)
	}

	// 校验磁盘剩余空间
	if diskFreeGB < m.minDiskGB {
		return nil, fmt.Errorf("%w: 当前磁盘剩余 %d GB，要求最小 %d GB", ErrInsufficientDisk, diskFreeGB, m.minDiskGB)
	}

	// 引用计数递增
	tpl.RefCount++
	return tpl, nil
}

// ReleaseInstance 销毁实例后释放模板引用计数
func (m *MemoryTemplateManager) ReleaseInstance(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil
	}

	if tpl.RefCount > 0 {
		tpl.RefCount--
	}
	return nil
}

// DeleteTemplate 删除模板，引用计数大于 0 时严格拒绝 (基底保护核心契约)
func (m *MemoryTemplateManager) DeleteTemplate(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil
	}

	if tpl.RefCount > 0 {
		return fmt.Errorf("%w: 仍有 %d 个实例正在引用此模板基底", ErrTemplateInUse, tpl.RefCount)
	}

	delete(m.templates, templateID)
	return nil
}
