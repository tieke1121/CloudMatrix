package store

import (
	"context"
	"path/filepath"
	"testing"

	"cloudmatrix/internal/domain"
)

func TestAppSessionPersistsOnlyMaskedAccountLabel(t *testing.T) {
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "session.db"))
	if err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	_, err = st.UpsertInstance(ctx, &domain.Instance{
		ID: "inst-1", HostID: "local", ProviderType: "mumu", ProviderInstanceID: "0",
		Name: "test", DesiredState: domain.StateReady, ObservedState: domain.StateReady,
	})
	if err != nil {
		t.Fatalf("写入实例失败: %v", err)
	}

	session := &domain.AppSession{
		InstanceID: "inst-1", PackageName: "com.haoyisheng.mobile.app",
		LoginStatus: domain.LoginStatusLoggedIn, AccountLabel: "****1234",
	}
	if err := st.UpsertAppSession(ctx, session); err != nil {
		t.Fatalf("写入登录状态失败: %v", err)
	}
	got, err := st.GetAppSession(ctx, "inst-1", "com.haoyisheng.mobile.app")
	if err != nil {
		t.Fatalf("读取登录状态失败: %v", err)
	}
	if got == nil || got.LoginStatus != domain.LoginStatusLoggedIn || got.AccountLabel != "****1234" {
		t.Fatalf("登录状态不匹配: %+v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("更新时间必须持久化")
	}
}

func TestAppSessionRejectsFullPhoneNumber(t *testing.T) {
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "sensitive.db"))
	if err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}
	defer st.Close()
	err = st.UpsertAppSession(context.Background(), &domain.AppSession{
		InstanceID: "inst-1", PackageName: "com.haoyisheng.mobile.app",
		LoginStatus: domain.LoginStatusLoggedIn, AccountLabel: "13800138000",
	})
	if err == nil {
		t.Fatal("存储层必须拒绝完整手机号")
	}
}
