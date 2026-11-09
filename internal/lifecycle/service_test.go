package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloudmatrix/internal/adb"
	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/store"
)

// fakeProvider 用于测试调度的虚拟厂商适配器
type fakeProvider struct {
	startCount int32
	stopCount  int32
	startDelay time.Duration
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Capabilities() *domain.ProviderCapabilities {
	return &domain.ProviderCapabilities{}
}
func (f *fakeProvider) Discover(ctx context.Context) ([]*domain.Instance, error) {
	state := domain.StateStopped
	if atomic.LoadInt32(&f.startCount) > atomic.LoadInt32(&f.stopCount) {
		state = domain.StateBooting
	}
	return []*domain.Instance{{ProviderInstanceID: "0", AdbSerial: "test-serial", ObservedState: state}}, nil
}

type readyADB struct{}

func (readyADB) CheckReadiness(ctx context.Context, serial string) (*adb.ReadinessResult, error) {
	return &adb.ReadinessResult{Ready: serial == "test-serial"}, nil
}
func (f *fakeProvider) Start(ctx context.Context, id string) error {
	atomic.AddInt32(&f.startCount, 1)
	if f.startDelay > 0 {
		time.Sleep(f.startDelay)
	}
	return nil
}
func (f *fakeProvider) Stop(ctx context.Context, id string) error {
	atomic.AddInt32(&f.stopCount, 1)
	return nil
}

// 1. 表驱动测试状态机转换合法性（特别是 error / unknown 绝不自动转 stopped）
func TestStateTransitions(t *testing.T) {
	cases := []struct {
		from  domain.InstanceState
		to    domain.InstanceState
		valid bool
	}{
		{domain.StateStopped, domain.StateStarting, true},
		{domain.StateStarting, domain.StateBooting, true},
		{domain.StateBooting, domain.StateReady, true},
		{domain.StateReady, domain.StateStopping, true},
		{domain.StateStopping, domain.StateStopped, true},
		// 严禁从 error 直接转换到 stopped
		{domain.StateError, domain.StateStopped, false},
		// error 仅允许显式 starting 或 stopping
		{domain.StateError, domain.StateStarting, true},
		{domain.StateError, domain.StateStopping, true},
	}

	for _, c := range cases {
		actual := CanTransition(c.from, c.to)
		if actual != c.valid {
			t.Errorf("状态流转校验不符: %s -> %s, 预期 %v, 实际 %v", c.from, c.to, c.valid, actual)
		}
	}
}

// 2. 验证并发启动 20 次至多触发 1 次厂商底层启动（V05 核心验收）
func TestConcurrentStartIdempotency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-lifecycle-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}
	defer st.Close()

	ctx := context.Background()

	// 插入一个测试实例
	inst := &domain.Instance{
		ID:                 "inst-test-01",
		HostID:             "local",
		ProviderType:       "fake",
		ProviderInstanceID: "0",
		Name:               "Test-Instance",
		DesiredState:       domain.StateStopped,
		ObservedState:      domain.StateStopped,
		AdbSerial:          "", // 启动后的 serial 必须来自厂商发现
	}
	if _, err := st.UpsertInstance(ctx, inst); err != nil {
		t.Fatalf("初始化实例失败: %v", err)
	}

	fakeProv := &fakeProvider{startDelay: 100 * time.Millisecond}
	adm := NewAdmissionController(10, 2)
	svc := NewService(st, fakeProv, readyADB{}, adm, time.Second, time.Second)

	// 模拟 20 个 goroutine 并发点击启动
	concurrency := 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, _ = svc.Start(ctx, "inst-test-01")
		}()
	}

	wg.Wait()

	calls := atomic.LoadInt32(&fakeProv.startCount)
	if calls != 1 {
		t.Fatalf("预期 20 次并发调用仅触发 1 次厂商启动，实际被调用了 %d 次", calls)
	}
}

// 3. 验证启动过程中上下文取消，最终状态绝不展示为 ready（V05 核心验收）
func TestStartCancelledNeverBecomesReady(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-lifecycle-cancel-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}
	defer st.Close()

	inst := &domain.Instance{
		ID:                 "inst-cancel-01",
		HostID:             "local",
		ProviderType:       "fake",
		ProviderInstanceID: "0",
		Name:               "Test-Cancel-Instance",
		DesiredState:       domain.StateStopped,
		ObservedState:      domain.StateStopped,
		AdbSerial:          "127.0.0.1:9999",
	}
	if _, err := st.UpsertInstance(context.Background(), inst); err != nil {
		t.Fatalf("初始化实例失败: %v", err)
	}

	fakeProv := &fakeProvider{startDelay: 200 * time.Millisecond}
	adm := NewAdmissionController(10, 2)
	svc := NewService(st, fakeProv, nil, adm, time.Second, time.Second)

	// 设置一个已取消的 Context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消

	_, startErr := svc.Start(ctx, "inst-cancel-01")
	if startErr == nil {
		t.Fatal("预期取消时返回错误，实际未报错")
	}

	// 检查数据库记录的最终状态
	updatedInst, err := st.GetInstance(context.Background(), "inst-cancel-01")
	if err != nil {
		t.Fatalf("获取实例失败: %v", err)
	}

	if updatedInst.ObservedState == domain.StateReady {
		t.Fatal("关键验收违背：启动被取消后最终状态绝对不能展示为 ready！")
	}
}

// 4. 验证资源配额达上限时拒绝启动并返回 RESOURCE_LIMIT
func TestAdmissionQuotaLimit(t *testing.T) {
	adm := NewAdmissionController(1, 1)

	// 实例 1 申请成功
	if err := adm.AcquireRunningQuota("inst-1"); err != nil {
		t.Fatalf("实例 1 申请配额失败: %v", err)
	}

	// 实例 2 申请超限
	err := adm.AcquireRunningQuota("inst-2")
	if err == nil {
		t.Fatal("超过配额应拒绝，实际未拒绝")
	}

	platErr, ok := err.(*domain.PlatformError)
	if !ok || platErr.Code != domain.ErrCodeResourceLimit {
		t.Fatalf("预期返回 RESOURCE_LIMIT 错误，实际为: %v", err)
	}

	// 释放后再次申请成功
	adm.ReleaseRunningQuota("inst-1")
	if err := adm.AcquireRunningQuota("inst-2"); err != nil {
		t.Fatalf("释放后实例 2 申请应成功，实际失败: %v", err)
	}
}

func TestLifecycleChecksVendorBeforeTreatingCachedStateAsSuccess(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	inst := &domain.Instance{ID: "instance", HostID: "local", ProviderType: "fake",
		ProviderInstanceID: "0", ObservedState: domain.StateReady}
	if _, err := st.UpsertInstance(ctx, inst); err != nil {
		t.Fatal(err)
	}
	prov := &fakeProvider{}
	svc := NewService(st, prov, readyADB{}, NewAdmissionController(1, 1), time.Second, time.Second)
	started, err := svc.Start(ctx, inst.ID)
	if err != nil || started.ObservedState != domain.StateReady || atomic.LoadInt32(&prov.startCount) != 1 {
		t.Fatalf("cached ready state bypassed launch or ADB verification: %+v %v", started, err)
	}
	inst.ObservedState = domain.StateStopped
	if _, err := st.UpsertInstance(ctx, inst); err != nil {
		t.Fatal(err)
	}
	stopped, err := svc.Stop(ctx, inst.ID)
	if err != nil || stopped.ObservedState != domain.StateStopped || atomic.LoadInt32(&prov.stopCount) != 1 {
		t.Fatalf("cached stopped state bypassed vendor stop: %+v %v", stopped, err)
	}
}
