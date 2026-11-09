package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloudmatrix/internal/domain"

	_ "modernc.org/sqlite"
)

// Store 持久化接口契约
type Store interface {
	Close() error
	UpsertInstance(ctx context.Context, inst *domain.Instance) (*domain.Instance, error)
	GetInstance(ctx context.Context, id string) (*domain.Instance, error)
	ListInstances(ctx context.Context) ([]*domain.Instance, error)
	CreateTask(ctx context.Context, task *domain.Task) error
	GetTask(ctx context.Context, id string) (*domain.Task, error)
	SaveTaskResult(ctx context.Context, task *domain.Task) error
	InterruptPendingTasks(ctx context.Context) error
	GetAppSession(ctx context.Context, instanceID, packageName string) (*domain.AppSession, error)
	UpsertAppSession(ctx context.Context, session *domain.AppSession) error
}

// SQLiteStore 基于 modernc.org/sqlite 的纯 Go SQLite 持久化层
type SQLiteStore struct {
	db      *sql.DB
	writeMu sync.Mutex
}

// NewSQLiteStore 初始化并建立 SQLite 存储层
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// 拼接 SQLite 参数，开启 WAL 模式、外键约束与 busy_timeout
	separator := "?"
	if strings.Contains(dbPath, "?") {
		separator = "&"
	}
	dsn := fmt.Sprintf("%s%s_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath, separator)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	// 限制单个写连接，避免并发写死锁
	db.SetMaxOpenConns(1)

	// 自动执行版本迁移检查
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库迁移校验未通过: %w", err)
	}

	return &SQLiteStore{
		db: db,
	}, nil
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// UpsertInstance 插入或更新实例，依据唯一索引防止重复创建
func (s *SQLiteStore) UpsertInstance(ctx context.Context, inst *domain.Instance) (*domain.Instance, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	if inst.CreatedAt.IsZero() {
		inst.CreatedAt = now
	}
	inst.UpdatedAt = now

	// 先尝试按唯一键查询既有记录的主键 ID
	querySQL := `
		SELECT id, created_at FROM instances 
		WHERE host_id = ? AND provider_type = ? AND provider_instance_id = ?
	`
	var existingID string
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, querySQL, inst.HostID, inst.ProviderType, inst.ProviderInstanceID).Scan(&existingID, &createdAt)

	if err == nil {
		// 记录已存在，保留原主键 ID 和创建时间，执行更新
		inst.ID = existingID
		inst.CreatedAt = createdAt

		updateSQL := `
			UPDATE instances SET 
				name = ?, desired_state = ?, observed_state = ?, adb_serial = ?, updated_at = ?
			WHERE id = ?
		`
		_, updateErr := s.db.ExecContext(ctx, updateSQL,
			inst.Name, string(inst.DesiredState), string(inst.ObservedState), inst.AdbSerial, inst.UpdatedAt, inst.ID)
		if updateErr != nil {
			return nil, fmt.Errorf("更新实例失败: %w", updateErr)
		}
		return inst, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("查询已有实例失败: %w", err)
	}

	// 不存在，插入新实例
	insertSQL := `
		INSERT INTO instances (
			id, host_id, provider_type, provider_instance_id, name, 
			desired_state, observed_state, adb_serial, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, insertErr := s.db.ExecContext(ctx, insertSQL,
		inst.ID, inst.HostID, inst.ProviderType, inst.ProviderInstanceID, inst.Name,
		string(inst.DesiredState), string(inst.ObservedState), inst.AdbSerial, inst.CreatedAt, inst.UpdatedAt)
	if insertErr != nil {
		return nil, fmt.Errorf("插入实例失败: %w", insertErr)
	}

	return inst, nil
}

// GetInstance 获取指定 ID 的实例
func (s *SQLiteStore) GetInstance(ctx context.Context, id string) (*domain.Instance, error) {
	querySQL := `
		SELECT id, host_id, provider_type, provider_instance_id, name, 
		       desired_state, observed_state, adb_serial, created_at, updated_at
		FROM instances WHERE id = ?
	`
	inst := &domain.Instance{}
	var desired, observed string
	err := s.db.QueryRowContext(ctx, querySQL, id).Scan(
		&inst.ID, &inst.HostID, &inst.ProviderType, &inst.ProviderInstanceID, &inst.Name,
		&desired, &observed, &inst.AdbSerial, &inst.CreatedAt, &inst.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewPlatformError(domain.ErrCodeInstanceNotFound, "指定实例不存在", false)
	}
	if err != nil {
		return nil, err
	}

	inst.DesiredState = domain.InstanceState(desired)
	inst.ObservedState = domain.InstanceState(observed)
	return inst, nil
}

// ListInstances 列出所有已纳管实例
func (s *SQLiteStore) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	querySQL := `
		SELECT id, host_id, provider_type, provider_instance_id, name, 
		       desired_state, observed_state, adb_serial, created_at, updated_at
		FROM instances ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Instance
	for rows.Next() {
		inst := &domain.Instance{}
		var desired, observed string
		if scanErr := rows.Scan(
			&inst.ID, &inst.HostID, &inst.ProviderType, &inst.ProviderInstanceID, &inst.Name,
			&desired, &observed, &inst.AdbSerial, &inst.CreatedAt, &inst.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		inst.DesiredState = domain.InstanceState(desired)
		inst.ObservedState = domain.InstanceState(observed)
		list = append(list, inst)
	}
	return list, rows.Err()
}

// CreateTask 创建异步任务与目标列表，严格校验幂等键
func (s *SQLiteStore) CreateTask(ctx context.Context, task *domain.Task) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	insertTaskSQL := `
		INSERT INTO tasks (id, type, status, idempotency_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	var keyParam interface{} = nil
	if task.IdempotencyKey != "" {
		keyParam = task.IdempotencyKey
	}

	_, taskErr := tx.ExecContext(ctx, insertTaskSQL,
		task.ID, task.Type, string(task.Status), keyParam, task.CreatedAt, task.UpdatedAt)
	if taskErr != nil {
		if strings.Contains(taskErr.Error(), "UNIQUE") {
			return domain.NewPlatformError(domain.ErrCodeConflict, "任务幂等键已存在，请勿重复提交", false)
		}
		return fmt.Errorf("创建任务主记录失败: %w", taskErr)
	}

	insertTargetSQL := `
		INSERT INTO task_targets (id, task_id, instance_id, status, exit_code, error_code, error_message, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	for _, target := range task.Targets {
		_, targetErr := tx.ExecContext(ctx, insertTargetSQL,
			target.ID, task.ID, target.InstanceID, string(target.Status), target.ExitCode,
			target.ErrorCode, target.ErrorMessage, target.StartedAt, target.EndedAt)
		if targetErr != nil {
			return fmt.Errorf("创建任务目标明细失败: %w", targetErr)
		}
	}

	return tx.Commit()
}

// GetTask 获取任务及其分目标执行详情
func (s *SQLiteStore) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	queryTaskSQL := `
		SELECT id, type, status, COALESCE(idempotency_key, ''), created_at, updated_at
		FROM tasks WHERE id = ?
	`
	task := &domain.Task{}
	var status string
	err := s.db.QueryRowContext(ctx, queryTaskSQL, id).Scan(
		&task.ID, &task.Type, &status, &task.IdempotencyKey, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewPlatformError(domain.ErrCodeInstanceNotFound, "指定任务不存在", false)
	}
	if err != nil {
		return nil, err
	}
	task.Status = domain.TaskStatus(status)

	// 查询目标明细
	queryTargetsSQL := `
		SELECT id, instance_id, status, exit_code, COALESCE(error_code, ''), COALESCE(error_message, ''), started_at, ended_at
		FROM task_targets WHERE task_id = ?
	`
	rows, err := s.db.QueryContext(ctx, queryTargetsSQL, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		target := &domain.TaskTarget{TaskID: id}
		var targetStatus string
		if scanErr := rows.Scan(
			&target.ID, &target.InstanceID, &targetStatus, &target.ExitCode,
			&target.ErrorCode, &target.ErrorMessage, &target.StartedAt, &target.EndedAt); scanErr != nil {
			return nil, scanErr
		}
		target.Status = domain.TaskStatus(targetStatus)
		task.Targets = append(task.Targets, target)
	}

	return task, rows.Err()
}
