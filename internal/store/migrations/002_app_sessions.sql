CREATE TABLE app_sessions (
    instance_id TEXT NOT NULL,
    package_name TEXT NOT NULL,
    login_status TEXT NOT NULL,
    account_label TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL,
    PRIMARY KEY(instance_id, package_name),
    FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
);
