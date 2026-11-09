package mumu

import (
	"os"
	"path/filepath"
	"testing"

	"cloudmatrix/internal/domain"
)

// 测试解析空列表
func TestParseEmptyList(t *testing.T) {
	list, err := ParseCLIOutput([]byte("[]"))
	if err != nil {
		t.Fatalf("解析空列表不应报错: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("预期 0 个实例，实际为: %d", len(list))
	}
}

func TestParseUnknownAndAmbiguousInstanceState(t *testing.T) {
	list, err := ParseCLIOutput([]byte(`[{"index": 0}]`))
	if err != nil || len(list) != 1 || list[0].ObservedState != domain.StateUnknown {
		t.Fatalf("missing running flag must not become stopped: %+v %v", list, err)
	}
	_, err = ParseCLIOutput([]byte(`[
		{"index": 0, "running": true, "adb_port": 16384},
		{"index": 1, "running": true, "adb_port": 16384}
	]`))
	if err == nil {
		t.Fatal("a shared ADB serial must not be accepted for two running instances")
	}
}

// 测试解析实例脱敏夹具
func TestParseInstancesTestData(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取目录失败: %v", err)
	}

	jsonPath := filepath.Join(wd, "testdata", "instances.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("读取测试夹具失败: %v", err)
	}

	instances, err := ParseCLIOutput(data)
	if err != nil {
		t.Fatalf("解析夹具失败: %v", err)
	}

	// 夹具中包含 4 个元素，其中一个为重复 ID，应去重保留 3 个
	if len(instances) != 3 {
		t.Fatalf("预期去重后 3 个实例，实际为: %d", len(instances))
	}

	// 验证 0 号实例
	inst0 := instances[0]
	if inst0.ProviderInstanceID != "0" {
		t.Errorf("实例 0 ID 错误: %s", inst0.ProviderInstanceID)
	}
	if inst0.AdbSerial != "127.0.0.1:16384" {
		t.Errorf("实例 0 AdbSerial 错误: %s", inst0.AdbSerial)
	}
	if inst0.ObservedState != domain.StateBooting {
		t.Errorf("实例 0 观测状态应为 booting，实际为: %s", inst0.ObservedState)
	}

	// 验证 1 号实例
	inst1 := instances[1]
	if inst1.ProviderInstanceID != "1" {
		t.Errorf("实例 1 ID 错误: %s", inst1.ProviderInstanceID)
	}
	if inst1.ObservedState != domain.StateStopped {
		t.Errorf("实例 1 状态应为 stopped，实际为: %s", inst1.ObservedState)
	}

	// 验证 2 号实例（测试缺省名称补全与端口推导）
	inst2 := instances[2]
	if inst2.Name != "MuMu-2" {
		t.Errorf("实例 2 名称应自动填充为 MuMu-2，实际为: %s", inst2.Name)
	}
	if inst2.AdbSerial != "127.0.0.1:16448" {
		t.Errorf("实例 2 端口转换错误: %s", inst2.AdbSerial)
	}
}

// 测试畸形 JSON 错误处理
func TestParseMalformedJSON(t *testing.T) {
	wd, _ := os.Getwd()
	jsonPath := filepath.Join(wd, "testdata", "malformed.json")
	data, _ := os.ReadFile(jsonPath)

	_, err := ParseCLIOutput(data)
	if err == nil {
		t.Fatal("预期格式损坏时报错，实际未报错")
	}
}

// 测试启停参数切片构造
func TestCommandFormatting(t *testing.T) {
	startArgs, err := FormatStartCommand("0")
	if err != nil || len(startArgs) != 3 || startArgs[2] != "0" {
		t.Fatalf("启动参数切片格式不符: %v", startArgs)
	}

	stopArgs, err := FormatStopCommand("1")
	if err != nil || len(stopArgs) != 3 || stopArgs[2] != "1" {
		t.Fatalf("停止参数切片格式不符: %v", stopArgs)
	}

	_, emptyErr := FormatStartCommand("")
	if emptyErr == nil {
		t.Fatal("空索引应返回错误")
	}
}
