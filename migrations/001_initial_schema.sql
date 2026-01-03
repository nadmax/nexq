CREATE TABLE task_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id VARCHAR(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    payload JSONB,
    priority INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    retry_count INTEGER DEFAULT 0,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    moved_to_dlq_at TIMESTAMPTZ,
    duration_ms INTEGER,
    worker_id VARCHAR(255),
    last_error TEXT,
    CONSTRAINT task_history_task_id_key UNIQUE(task_id)
);

CREATE INDEX idx_task_history_type ON task_history(type);
CREATE INDEX idx_task_history_status ON task_history(status);
CREATE INDEX idx_task_history_created_at ON task_history(created_at DESC);
CREATE INDEX idx_task_history_completed_at ON task_history(completed_at DESC) WHERE completed_at IS NOT NULL;
CREATE INDEX idx_task_history_type_status ON task_history(type, status);
CREATE INDEX idx_task_history_payload ON task_history USING gin(payload);

CREATE TABLE task_execution_log (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(255) NOT NULL,
    attempt_number INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    error_message TEXT,
    worker_id VARCHAR(255),
    FOREIGN KEY (task_id) REFERENCES task_history(task_id) ON DELETE CASCADE
);

CREATE INDEX idx_task_execution_log_task_id ON task_execution_log(task_id);
CREATE INDEX idx_task_execution_log_started_at ON task_execution_log(started_at DESC);

CREATE VIEW task_stats AS
SELECT 
    type,
    status,
    COUNT(*) as count,
    AVG(duration_ms) as avg_duration_ms,
    MAX(duration_ms) as max_duration_ms,
    MIN(duration_ms) as min_duration_ms,
    AVG(retry_count) as avg_retries
FROM task_history
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY type, status;

CREATE VIEW recent_tasks AS
SELECT 
    task_id,
    type,
    status,
    created_at,
    completed_at,
    duration_ms,
    retry_count,
    failure_reason
FROM task_history
ORDER BY created_at DESC
LIMIT 100;
