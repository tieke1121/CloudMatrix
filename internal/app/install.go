package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/task"

	"github.com/google/uuid"
)

// InstallAPK persists the selected targets before handing them to the queue.
func (s *Service) InstallAPK(ctx context.Context, req InstallRequest) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	if len(req.InstanceIDs) == 0 {
		return OperationResponse{}, fmt.Errorf("请选择至少一个实例")
	}
	path := strings.TrimSpace(req.APKPath)
	if !filepath.IsAbs(path) {
		return OperationResponse{}, fmt.Errorf("请输入 APK 的绝对路径")
	}
	batch := &domain.Task{ID: uuid.NewString(), Type: "install_apk", Status: domain.TaskQueued}
	seen := make(map[string]bool, len(req.InstanceIDs))
	for _, id := range req.InstanceIDs {
		if strings.TrimSpace(id) == "" {
			return OperationResponse{}, fmt.Errorf("实例 ID 不能为空")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		batch.Targets = append(batch.Targets, &domain.TaskTarget{
			ID: uuid.NewString(), TaskID: batch.ID, InstanceID: id, Status: domain.TaskQueued,
		})
	}
	stagedPath, identity, err := prepareGoodDoctorAPK(
		ctx, path, s.cfg.DataDir, s.cfg.Aapt2.Executable, s.cfg.Timeouts.QueryTimeout, task.InspectAPKIdentity,
	)
	if err != nil {
		return OperationResponse{}, err
	}

	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if s.closing {
		return OperationResponse{}, errors.Join(fmt.Errorf("应用正在退出"), os.Remove(stagedPath))
	}
	if err := s.store.CreateTask(ctx, batch); err != nil {
		return OperationResponse{}, errors.Join(err, os.Remove(stagedPath))
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.activeTasks[batch.ID] = cancel
	s.taskWorkers.Add(1)
	go s.runInstall(runCtx, cancel, batch, stagedPath, identity)
	return OperationResponse{Success: true, Message: "APK 任务已入队", TaskID: batch.ID}, nil
}

type apkIdentityInspector func(context.Context, string, string, time.Duration) (*task.APKIdentity, error)

func prepareGoodDoctorAPK(
	ctx context.Context,
	path, dataDir, aapt2Executable string,
	timeout time.Duration,
	inspect apkIdentityInspector,
) (stagedPath string, identity *task.APKIdentity, err error) {
	meta, err := task.ValidateAPKFile(path, 1024*1024*1024)
	if err != nil {
		return "", nil, fmt.Errorf("APK 校验失败: %w", err)
	}
	stagedPath, err = task.StageAPK(meta, dataDir)
	if err != nil {
		return "", nil, fmt.Errorf("准备已校验的 APK: %w", err)
	}
	cleanupPath := stagedPath
	keepStaged := false
	defer func() {
		if !keepStaged {
			err = errors.Join(err, os.Remove(cleanupPath))
		}
	}()
	identity, err = inspect(ctx, aapt2Executable, stagedPath, timeout)
	if err != nil {
		return "", nil, fmt.Errorf("APK 身份校验失败: %w", err)
	}
	if identity.PackageName != GoodDoctorPackageName {
		return "", nil, domain.NewPlatformError(
			domain.ErrCodeInvalidArgument,
			fmt.Sprintf("仅允许安装好医生 APK，实际包名为 %s", identity.PackageName),
			false,
		)
	}
	keepStaged = true
	return stagedPath, identity, nil
}

func (s *Service) runInstall(ctx context.Context, cancel context.CancelFunc, batch *domain.Task, path string, identity *task.APKIdentity) {
	defer s.taskWorkers.Done()
	defer cancel()
	defer func() {
		if err := os.Remove(path); err != nil {
			log.Printf("任务 %s 的临时 APK 清理失败", batch.ID)
		}
	}()
	defer func() {
		s.taskMu.Lock()
		delete(s.activeTasks, batch.ID)
		s.taskMu.Unlock()
	}()

	batch.Status = domain.TaskRunning
	batch.UpdatedAt = time.Now().UTC()
	saveCtx, endSave := context.WithTimeout(context.Background(), 5*time.Second)
	err := s.store.SaveTaskResult(saveCtx, batch)
	endSave()
	if err != nil {
		log.Printf("任务 %s 未执行：无法保存运行状态", batch.ID)
		return
	}
	s.taskQueue.Execute(ctx, batch, func(targetCtx context.Context, target *domain.TaskTarget) error {
		inst, err := s.managedInstance(targetCtx, target.InstanceID)
		if err != nil {
			return err
		}
		if err := s.checkCurrentSerial(targetCtx, inst); err != nil {
			return err
		}
		if err := task.InstallAPKOnDevice(targetCtx, s.adbClient, inst.AdbSerial, path, s.cfg.Timeouts.ApkInstallTimeout); err != nil {
			return err
		}
		installed, err := s.adbClient.GetPackageInfo(targetCtx, inst.AdbSerial, identity.PackageName, s.cfg.Timeouts.ShellTimeout)
		if err != nil {
			return fmt.Errorf("安装后确认失败: %w", err)
		}
		if !installed.Installed || installed.VersionCode != identity.VersionCode {
			return domain.NewPlatformError(domain.ErrCodeConflict, "设备未确认安装相同版本的好医生 APK", true)
		}
		return s.store.UpsertAppSession(targetCtx, &domain.AppSession{
			InstanceID: target.InstanceID, PackageName: GoodDoctorPackageName,
			LoginStatus: domain.LoginStatusNotLoggedIn,
		})
	})
	saveCtx, endSave = context.WithTimeout(context.Background(), 5*time.Second)
	defer endSave()
	if err := s.store.SaveTaskResult(saveCtx, batch); err != nil {
		log.Printf("任务 %s 的结果写入失败", batch.ID)
	}
}

func (s *Service) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	result, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, active := s.activeTasks[id]; !active && (result.Status == domain.TaskQueued || result.Status == domain.TaskRunning) {
		return nil, fmt.Errorf("任务 %s 的执行状态未能写入数据库", id)
	}
	return result, nil
}

func (s *Service) CancelTask(id string) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	cancel := s.activeTasks[id]
	if cancel == nil {
		return fmt.Errorf("任务已结束或不存在")
	}
	cancel()
	return nil
}

func (s *Service) Shutdown() {
	s.taskMu.Lock()
	s.closing = true
	for _, cancel := range s.activeTasks {
		cancel()
	}
	s.taskMu.Unlock()
	s.taskWorkers.Wait()
	s.instanceMu.Lock()
	defer s.instanceMu.Unlock()
	if s.streamManager != nil {
		for _, session := range s.streamManager.ListSessions() {
			if err := s.streamManager.StopSession(session.InstanceID); err != nil {
				log.Printf("关闭投屏失败: %v", err)
			}
		}
	}
}
