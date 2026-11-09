package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
)

var maskedAccountLabelPattern = regexp.MustCompile(`^\*{4}\d{4}$`)

func (s *SQLiteStore) GetAppSession(ctx context.Context, instanceID, packageName string) (*domain.AppSession, error) {
	session := &domain.AppSession{}
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT instance_id, package_name, login_status, account_label, updated_at
		FROM app_sessions WHERE instance_id = ? AND package_name = ?`,
		instanceID, packageName,
	).Scan(&session.InstanceID, &session.PackageName, &status, &session.AccountLabel, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取应用登录状态: %w", err)
	}
	session.LoginStatus = domain.LoginStatus(status)
	return session, nil
}

func (s *SQLiteStore) UpsertAppSession(ctx context.Context, session *domain.AppSession) error {
	if session == nil || strings.TrimSpace(session.InstanceID) == "" || strings.TrimSpace(session.PackageName) == "" {
		return domain.NewPlatformError(domain.ErrCodeInvalidArgument, "应用登录状态缺少实例或包名", false)
	}
	if session.AccountLabel != "" && !maskedAccountLabelPattern.MatchString(session.AccountLabel) {
		return domain.NewPlatformError(domain.ErrCodeInvalidArgument, "账号标记只能是掩码后的手机号后 4 位", false)
	}
	if !session.LoginStatus.Valid() {
		return domain.NewPlatformError(domain.ErrCodeInvalidArgument, "应用登录状态不合法", false)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	session.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_sessions (instance_id, package_name, login_status, account_label, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(instance_id, package_name) DO UPDATE SET
			login_status = excluded.login_status,
			account_label = excluded.account_label,
			updated_at = excluded.updated_at`,
		session.InstanceID, session.PackageName, string(session.LoginStatus), session.AccountLabel, session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存应用登录状态: %w", err)
	}
	return nil
}
