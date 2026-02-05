class ToastManager {
    constructor() {
        this.container = this.createContainer();
        document.body.appendChild(this.container);
    }

    createContainer() {
        const container = document.createElement('div');
        container.className = 'toast-container';
        return container;
    }

    show(message, type = 'info', duration = 4000) {
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;

        const icon = this.getIcon(type);
        toast.innerHTML = `
            <span class="toast-icon">${icon}</span>
            <span class="toast-message">${message}</span>
            <button class="toast-close" aria-label="Close">&times;</button>
        `;

        this.container.appendChild(toast);

        setTimeout(() => toast.classList.add('toast-show'), 10);

        const timeoutId = setTimeout(() => this.dismiss(toast), duration);

        toast.querySelector('.toast-close').addEventListener('click', () => {
            clearTimeout(timeoutId);
            this.dismiss(toast);
        });

        return toast;
    }

    dismiss(toast) {
        toast.classList.remove('toast-show');
        setTimeout(() => toast.remove(), 300);
    }

    getIcon(type) {
        const icons = {
            success: '✓',
            error: '✕',
            warning: '⚠',
            info: 'ℹ'
        };
        return icons[type] || icons.info;
    }

    success(message, duration) {
        return this.show(message, 'success', duration);
    }

    error(message, duration) {
        return this.show(message, 'error', duration);
    }

    warning(message, duration) {
        return this.show(message, 'warning', duration);
    }

    info(message, duration) {
        return this.show(message, 'info', duration);
    }
}

class ConfirmDialog {
    constructor() {
        this.overlay = this.createOverlay();
        document.body.appendChild(this.overlay);
    }

    createOverlay() {
        const overlay = document.createElement('div');
        overlay.className = 'confirm-overlay';
        return overlay;
    }

    show(options) {
        return new Promise((resolve) => {
            const {
                title = 'Confirm',
                message,
                confirmText = 'Confirm',
                cancelText = 'Cancel',
                type = 'default'
            } = options;

            const dialog = document.createElement('div');
            dialog.className = `confirm-dialog confirm-${type}`;
            dialog.innerHTML = `
                <div class="confirm-header">
                    <h3>${title}</h3>
                </div>
                <div class="confirm-body">
                    <p>${message}</p>
                </div>
                <div class="confirm-footer">
                    <button class="confirm-btn confirm-btn-cancel">${cancelText}</button>
                    <button class="confirm-btn confirm-btn-confirm confirm-btn-${type}">${confirmText}</button>
                </div>
            `;

            this.overlay.appendChild(dialog);
            this.overlay.classList.add('confirm-show');

            setTimeout(() => dialog.classList.add('confirm-dialog-show'), 10);

            const cleanup = () => {
                dialog.classList.remove('confirm-dialog-show');
                this.overlay.classList.remove('confirm-show');
                setTimeout(() => {
                    dialog.remove();
                }, 300);
            };

            dialog.querySelector('.confirm-btn-cancel').addEventListener('click', () => {
                cleanup();
                resolve(false);
            });

            dialog.querySelector('.confirm-btn-confirm').addEventListener('click', () => {
                cleanup();
                resolve(true);
            });

            this.overlay.addEventListener('click', (e) => {
                if (e.target === this.overlay) {
                    cleanup();
                    resolve(false);
                }
            }, { once: true });

            const escHandler = (e) => {
                if (e.key === 'Escape') {
                    cleanup();
                    resolve(false);
                    document.removeEventListener('keydown', escHandler);
                }
            };
            document.addEventListener('keydown', escHandler);
        });
    }

    confirm(message, title) {
        return this.show({ message, title, type: 'primary' });
    }

    danger(message, title) {
        return this.show({
            message,
            title: title || 'Warning',
            type: 'danger',
            confirmText: 'Delete'
        });
    }

    warning(message, title) {
        return this.show({
            message,
            title: title || 'Confirm Action',
            type: 'warning',
            confirmText: 'Continue'
        });
    }
}

const toast = new ToastManager();
const confirm = new ConfirmDialog();
const API_URL = '/api';
const codeExample = [
    { report_type: "task_summary", start_time: "2026-01-01T00:00:00Z", end_time: "2026-01-04T23:59:59Z", format: "csv", output_path: "./reports", schedule_in: 3600 },
    { image_url: "https://example.com/image.jpg", operations: ["resize", "compress"] }
];
let editor;
let editorsExample = [];

require.config({ paths: { vs: 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.53.0/min/vs' } });
require(['vs/editor/editor.main'], function () {
    editor = monaco.editor.create(document.getElementById('editor'), {
        value: JSON.stringify({
            report_type: "task_summary",
            start_time: "2026-01-01T00:00:00Z",
            end_time: "2026-01-04T23:59:59Z",
            format: "csv",
            output_path: "./reports",
            schedule_in: 3600
        }, null, 2),
        language: 'json',
        theme: 'vs',
        minimap: { enabled: false },
        fontSize: 14,
        lineNumbers: 'on',
        roundedSelection: true,
        scrollBeyondLastLine: false,
        automaticLayout: true,
        padding: { top: 10, bottom: 10 },
        overviewRulerBorder: false,
        hideCursorInOverviewRuler: true,
        autoClosingBrackets: 'always',
        renderLineHighlight: 'none'
    });
    codeExample.forEach((code, i) => {
        const exampleEditor = monaco.editor.create(document.getElementById(`example-${i}`), {
            value: JSON.stringify(code, null, 2),
            language: 'json',
            theme: 'vs',
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: 'off',
            readOnly: true,
            scrollBeyondLastLine: false,
            automaticLayout: true,
            padding: { top: 10, bottom: 10 },
            overviewRulerBorder: false,
            hideCursorInOverviewRuler: true,
            renderLineHighlight: 'none',
            scrollbar: {
                vertical: 'hidden',
                horizontal: 'hidden'
            }
        });
        editorsExample.push(exampleEditor);
    });
});

function switchTab(tabName, element) {
    document.querySelectorAll('.tab').forEach(tab => tab.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('active'));

    element.classList.add('active');
    document.getElementById(`${tabName}-tab`).classList.add('active');
    if (tabName === 'dlq') {
        loadDLQTasks();
        loadDLQStats();
    } else if (tabName == 'history') {
        loadRecentHistory();
        loadHistoryStats();
    } else if (tabName == 'reports') {
        loadReports();
    } else {
        loadStats();
        loadTasks();
    }
}

function copyCode(button, index) {
    const code = editorsExample[index].getValue();

    navigator.clipboard.writeText(code).then(() => {
        button.textContent = 'Copied!';
        button.classList.add('copied');
        setTimeout(() => {
            button.textContent = 'Copy';
            button.classList.remove('copied');
        }, 2000);
    });
}

function getPriorityLabel(priority) {
    const labels = ['low', 'medium', 'high'];
    return labels[priority] || 'medium';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

document.getElementById('taskForm').addEventListener('submit', async (e) => {
    e.preventDefault();

    try {
        const payload = JSON.parse(editor.getValue());
        const data = {
            type: document.getElementById('taskType').value,
            payload: payload,
            priority: parseInt(document.getElementById('priority').value)
        };

        await fetch(`${API_URL}/tasks`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        toast.success('Task created successfully');
        refreshCurrentTab();
        setTimeout(refreshCurrentTab, 300);
    } catch (err) {
        toast.error('Error creating task: ' + err.message);
    }
});

async function cancelTask(taskId) {
    const confirmed = await confirm.show({
        title: 'Cancel Task?',
        message: 'This task will be marked as cancelled and stopped if running.',
        confirmText: 'Yes, Cancel Task',
        cancelText: 'No, Keep Task',
        type: 'primary'
    });

    if (!confirmed) return;

    try {
        const response = await fetch(`${API_URL}/tasks/cancel/${taskId}`, {
            method: 'POST'
        });

        if (response.ok) {
            toast.success('Task cancelled successfully');
            refreshCurrentTab();
        } else {
            const error = await response.json();
            toast.error('Failed to cancel task: ' + (error.error || 'Unknown error'));
        }
    } catch (err) {
        toast.error('Error cancelling task: ' + err.message);
    }
}


async function loadTasks() {
    try {
        const response = await fetch(`${API_URL}/tasks`);
        if (!response.ok) {
            const err = await response.json();
            throw Error(err.error || 'failed to load tasks');
        }

        const tasks = await response.json();
        const taskList = document.getElementById('taskList');
        if (!tasks || tasks.length === 0) {
            taskList.innerHTML = '<div class="empty-state">No tasks yet. Create one above!</div>';
            return;
        }

        taskList.innerHTML = tasks.map(task => `
            <div class="task">
                <div class="task-info">
                    <strong>${task.type}</strong>
                    <span class="priority ${getPriorityLabel(task.priority)}">${getPriorityLabel(task.priority).toUpperCase()}</span>
                    <small>${task.id}</small>
                </div>
                <div>
                    <small>Created: ${new Date(task.created_at).toLocaleString()}</small>
                    ${task.scheduled_at !== task.created_at ? `<br><small>Scheduled: ${new Date(task.scheduled_at).toLocaleString()}</small>` : ''}
                </div>
                <div class="status ${task.status}">${task.status}</div>
                <div>
                    <small>${task.retry_count || 0}/${task.max_retries || 3} retries</small>
                    ${task.error ? `<br><small style="color: #dc3545;">${task.error}</small>` : ''}
                    ${(task.status === 'pending' || task.status === 'running') ? `
                        <br><button class="cancel-btn" onclick="cancelTask('${task.id}')">Cancel</button>
                    ` : ''}
                </div>
            </div>
        `).join('');
    } catch (err) {
        console.error('Error loading tasks:', err);
    }
}

async function loadStats() {
    try {
        const response = await fetch(`${API_URL}/dashboard/stats`);
        if (!response.ok) {
            const err = await response.json();
            throw Error(err.error || 'failed to load stats');
        }

        const stats = await response.json();
        document.getElementById('stats').innerHTML = `
            <div class="stats-grid">
                <div class="stat-card">
                    <h3>Total</h3>
                    <p class="stat-number">${stats.total_tasks || 0}</p>
                </div>
                <div class="stat-card">
                    <h3>Pending</h3>
                    <p class="stat-number">${stats.pending_tasks || 0}</p>
                </div>
                <div class="stat-card">
                    <h3>Running</h3>
                    <p class="stat-number">${stats.running_tasks || 0}</p>
                </div>
                <div class="stat-card">
                    <h3>Completed</h3>
                    <p class="stat-number">${stats.completed_tasks || 0}</p>
                </div>
                <div class="stat-card">
                    <h3>Failed</h3>
                    <p class="stat-number">${stats.failed_tasks || 0}</p>
                </div>
                <div class="stat-card">
                    <h3>Cancelled</h3>
                    <p class="stat-number">${stats.cancelled_tasks || 0}</p>
                </div>
            </div>
            <br>
            ${stats.average_wait_time ? `<p><strong>Average Wait Time:</strong> ${stats.average_wait_time}</p>` : ''}
        `;
    } catch (err) {
        console.error('Error loading stats:', err);
    }
}

async function loadDLQTasks() {
    try {
        const response = await fetch(`${API_URL}/dlq/tasks`);
        if (!response.ok) {
            const err = await response.json();
            throw new Error(err.error || 'failed to load dead letter tasks');
        }

        const tasks = await response.json();
        const dlqList = document.getElementById('dlqList');
        if (!dlqList) return;

        if (!tasks || tasks.length === 0) {
            dlqList.innerHTML = `
                <div class="dlq-empty-state">
                    <div class="dlq-empty-state-icon">✅</div>
                    <h3>No failed tasks</h3>
                    <p>All tasks are running smoothly!</p>
                </div>
            `;
            return;
        }

        dlqList.innerHTML = tasks.map((task, index) => {
            const taskId = task.id.replace(/'/g, "\\'").replace(/"/g, "&quot;");
            const safeId = `dlq-item-${index}`;
            const failureReason = task.failure_reason ? task.failure_reason : '';
            const payloadJson = task.payload ? JSON.stringify(task.payload, null, 2) : '';
            const movedToDLQAt = task.moved_to_dlq_at ? new Date(task.moved_to_dlq_at).toLocaleString() : 'Unknown';
            const createdAt = task.created_at ? new Date(task.created_at).toLocaleString() : 'Unknown';
            const hasPayload = task.payload && Object.keys(task.payload).length > 0;
            const escapedFailureReason = failureReason ? escapeHtml(failureReason) : '';
            const escapedPayloadJson = hasPayload ? escapeHtml(payloadJson) : '';

            return `
                <div class="dlq-task" data-task-id="${taskId}" data-safe-id="${safeId}">
                    <div class="dlq-task-header">
                        <div class="dlq-task-main-info">
                            <div class="dlq-task-type">${escapeHtml(task.type)}</div>
                            <span class="priority ${getPriorityLabel(task.priority)}">${getPriorityLabel(task.priority).toUpperCase()}</span>
                        </div>
                        <div class="dlq-task-timestamps">
                            <small><strong>Failed:</strong> ${movedToDLQAt}</small>
                            <small><strong>Created:</strong> ${createdAt}</small>
                        </div>
                    </div>
                    
                    <div class="dlq-task-id">ID: ${escapeHtml(task.id)}</div>
                    
                    ${task.failure_reason ? `
                        <div class="dlq-failure-section">
                            <div class="dlq-failure-header" onclick="toggleFailureReason('${safeId}')">
                                <div class="dlq-failure-title">Failure Reason</div>
                                <button class="dlq-failure-toggle" aria-label="Toggle failure details" type="button">
                                    <span id="toggle-${safeId}">▼</span>
                                </button>
                            </div>
                            <div class="dlq-failure-content" id="failure-${safeId}">
                                <div class="dlq-failure-reason">${escapedFailureReason}</div>
                            </div>
                        </div>
                    ` : ''}
                    
                    ${hasPayload ? `
                        <div class="dlq-payload-section">
                            <button class="dlq-payload-toggle" onclick="togglePayload('${safeId}')" type="button">
                                <span>View Payload</span>
                                <span id="payload-toggle-${safeId}">▶</span>
                            </button>
                            <div class="dlq-payload-content" id="payload-${safeId}">
                                <div class="dlq-payload-json">${escapedPayloadJson}</div>
                            </div>
                        </div>
                    ` : ''}
                    
                    <div class="dlq-task-footer">
                        <div class="dlq-retry-info">
                            <div class="dlq-retry-badge">
                                <span>#</span>
                                <span>Retries: ${task.retry_count || 0}/${task.max_retries || 3}</span>
                            </div>
                        </div>
                        <div class="dlq-actions">
                            <button onclick="retryTask('${taskId}')" title="Retry this task">
                                Retry Task
                            </button>
                            <button class="danger" onclick="purgeTask('${taskId}')" title="Permanently delete this task">
                                Delete Permanently
                            </button>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    } catch (err) {
        console.error('Error loading DLQ tasks:', err);
        document.getElementById('dlqList').innerHTML = `
            <div class="dlq-empty-state">
                <div class="dlq-empty-state-icon">❌</div>
                <h3>Error loading DLQ tasks</h3>
                <p>${err.message || 'Failed to load failed tasks'}</p>
            </div>
        `;
    }
}

function toggleFailureReason(safeId) {
    const content = document.getElementById(`failure-${safeId}`);
    const toggle = document.getElementById(`toggle-${safeId}`);

    if (!content || !toggle) return;

    if (content.classList.contains('expanded')) {
        content.classList.remove('expanded');
        toggle.textContent = '▼';

        return;
    }

    content.classList.add('expanded');
    toggle.textContent = '▲';
}

function togglePayload(safeId) {
    const content = document.getElementById(`payload-${safeId}`);
    const toggle = document.getElementById(`payload-toggle-${safeId}`);

    if (!content || !toggle) return;

    if (content.classList.contains('expanded')) {
        content.classList.remove('expanded');
        toggle.textContent = '▶';

        return;
    }

    content.classList.add('expanded');
    toggle.textContent = '▼';
}

async function loadDLQStats() {
    try {
        const response = await fetch(`${API_URL}/dlq/stats`);
        if (!response.ok) {
            const err = await response.json();
            throw Error(err.error || 'failed to load dead letter stats');
        }

        const stats = await response.json();
        const totalTasks = stats.total_tasks || 0;

        let statsHtml = `
            <div class="dlq-stats-enhanced">
                <div class="dlq-stat-card">
                    <h3>Total Failed Tasks</h3>
                    <p class="dlq-stat-number">${totalTasks}</p>
                    <div class="dlq-stat-time">${totalTasks === 1 ? 'task' : 'tasks'} in DLQ</div>
                </div>
        `;

        if (stats.oldest_task_time) {
            const oldestDate = new Date(stats.oldest_task_time);
            const now = new Date();
            const ageMs = now - oldestDate;
            const ageDays = Math.floor(ageMs / (1000 * 60 * 60 * 24));
            const ageHours = Math.floor((ageMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));

            statsHtml += `
                <div class="dlq-stat-card">
                    <h3>Oldest Failed Task</h3>
                    <p class="dlq-stat-number" style="font-size: 18px;">${ageDays}d ${ageHours}h</p>
                    <div class="dlq-stat-time">${oldestDate.toLocaleString()}</div>
                </div>
            `;
        }

        if (stats.newest_task_time) {
            const newestDate = new Date(stats.newest_task_time);
            const now = new Date();
            const ageMs = now - newestDate;
            const ageMinutes = Math.floor(ageMs / (1000 * 60));
            const ageHours = Math.floor(ageMs / (1000 * 60 * 60));

            let ageText;
            if (ageMinutes < 60) {
                ageText = `${ageMinutes}m ago`;
            } else if (ageHours < 24) {
                ageText = `${ageHours}h ago`;
            } else {
                ageText = `${Math.floor(ageHours / 24)}d ago`;
            }

            statsHtml += `
                <div class="dlq-stat-card">
                    <h3>Newest Failed Task</h3>
                    <p class="dlq-stat-number" style="font-size: 18px;">${ageText}</p>
                    <div class="dlq-stat-time">${newestDate.toLocaleString()}</div>
                </div>
            `;
        }

        statsHtml += `</div>`;

        document.getElementById('dlqStats').innerHTML = statsHtml;
    } catch (err) {
        console.error('Error loading DLQ stats:', err);
        document.getElementById('dlqStats').innerHTML = `
            <div class="dlq-empty-state">
                <div class="dlq-empty-state-icon">⚠️</div>
                <h3>Error loading statistics</h3>
                <p>${err.message || 'Failed to load DLQ statistics'}</p>
            </div>
        `;
    }
}

async function loadHistoryStats() {
    const hours = document.getElementById('historyHours')?.value || 24;
    try {
        const response = await fetch(`${API_URL}/history/stats?hours=${hours}`);
        if (!response.ok) {
            if (response.status === 503) {
                document.getElementById('historyStats').innerHTML = `
                    <div class="empty-state">
                        <p>History not configured</p>
                        <p style="font-size: 14px; color: var(--muted);">
                            Need a database to track task history and performance metrics
                        </p>
                    </div>
                `;
                return;
            }
            throw new Error('Failed to load history stats');
        }

        const stats = await response.json();
        if (!stats || stats.length === 0) {
            document.getElementById('historyStats').innerHTML = `
                <div class="empty-state">No task history in the selected time range</div>
            `;
            return;
        }

        const statsByType = {};
        stats.forEach(stat => {
            if (!statsByType[stat.type]) {
                statsByType[stat.type] = [];
            }
            statsByType[stat.type].push(stat);
        });

        const html = `
            <div class="history-stats-container">
                ${Object.entries(statsByType).map(([type, typeStats]) => {
            const totalCount = typeStats.reduce((sum, s) => sum + s.count, 0);
            const completed = typeStats.find(s => s.status === 'completed');
            const failed = typeStats.find(s => s.status === 'failed');

            return `
                <div class="type-stats">
                    <h4>${type}</h4>
                    <div style="margin-bottom: 12px; color: var(--muted); font-size: 13px;">
                        Total: ${totalCount} tasks
                    </div>
                    ${typeStats.map(stat => `
                        <div class="stat-row">
                            <span class="status ${stat.status}">${stat.status}</span>
                            <span>${stat.count} tasks</span>
                            ${stat.avg_duration_ms ? `
                                <span class="stat-metric">~${Math.round(stat.avg_duration_ms)}ms</span>
                             ` : ''}
                        </div>
                        `).join('')}
                        ${completed && completed.avg_duration_ms ? `
                            <div style="padding-top: 12px; font-size: 12px; color: var(--muted);">
                                <div>Avg Duration: ${Math.round(completed.avg_duration_ms)}ms</div>
                                ${completed.max_duration_ms ? `<div>Max Duration: ${Math.round(completed.max_duration_ms)}ms</div>` : ''}
                                ${completed.avg_retries ? `<div>Avg Retries: ${completed.avg_retries.toFixed(2)}</div>` : ''}
                            </div>
                        ` : ''}
                        ${failed && failed.count > 0 ? `
                            <div style="padding-top: 12px; font-size: 12px; color: var(--error, #ef4444); border-top: 1px solid var(--border); margin-top: 12px;">
                                <div style="font-weight: 500; margin-bottom: 4px;">Failed Tasks</div>
                                <div>Count: ${failed.count}</div>
                                ${failed.avg_duration_ms ? `<div>Avg Duration: ${Math.round(failed.avg_duration_ms)}ms</div>` : ''}
                                ${failed.max_duration_ms ? `<div>Max Duration: ${Math.round(failed.max_duration_ms)}ms</div>` : ''}
                                ${failed.avg_retries ? `<div>Avg Retries: ${failed.avg_retries.toFixed(2)}</div>` : ''}
                            </div>
                        ` : ''}
                        </div>
                    `;
        }).join('')}
            </div>
        `;

        document.getElementById('historyStats').innerHTML = html;
    } catch (err) {
        console.error('Error loading history stats:', err);
        document.getElementById('historyStats').innerHTML = `
            <div class="empty-state">Error loading history stats</div>
        `;
    }
}

async function loadRecentHistory() {
    const limit = document.getElementById('historyLimit')?.value || 50;

    try {
        const response = await fetch(`${API_URL}/history/recent?limit=${limit}`);
        if (!response.ok) {
            if (response.status === 503) {
                document.getElementById('historyList').innerHTML = `
                    <div class="empty-state">PostgreSQL history not configured</div>
                `;
                return;
            }
            throw new Error('Failed to load recent history');
        }

        const tasks = await response.json();
        if (!tasks || tasks.length === 0) {
            document.getElementById('historyList').innerHTML = `
                <div class="empty-state">No historical tasks yet</div>
            `;
            return;
        }

        document.getElementById('historyList').innerHTML = tasks.map(task => {
            const duration = task.duration_ms ? `${task.duration_ms}ms` : 'N/A';
            const created = new Date(task.created_at).toLocaleString();
            const completed = task.completed_at ? new Date(task.completed_at).toLocaleString() : 'N/A';

            return `
                <div class="task task-clickable" onclick="viewTaskHistory('${task.task_id}')">
                    <div class="task-info">
                        <strong>${task.type}</strong>
                        <br>
                        <small style="color: var(--muted);">${task.task_id}</small>
                    </div>
                    <div>
                        <small>Created: ${created}</small>
                        ${task.completed_at ? `<br><small>Completed: ${completed}</small>` : ''}
                    </div>
                    <div class="status ${task.status}">${task.status}</div>
                    <div>
                        ${task.duration_ms ? `<small>Duration: ${duration}</small><br>` : ''}
                        <small>Retries: ${task.retry_count}</small>
                        ${task.failure_reason ? `<br><small style="color: var(--danger);">${task.failure_reason}</small>` : ''}
                    </div>
                </div>
            `;
        }).join('');
    } catch (err) {
        console.error('Error loading recent history:', err);
        document.getElementById('historyList').innerHTML = `
            <div class="empty-state">Error loading history</div>
        `;
    }
}

async function viewTaskHistory(taskId) {
    try {
        const response = await fetch(`${API_URL}/history/task/${taskId}`);
        if (!response.ok) {
            throw new Error('Failed to load task history');
        }

        const history = await response.json();
        if (!history || history.length === 0) {
            toast.info('No execution history found for this task');
            return;
        }

        const modal = document.getElementById('executionModal');
        const historyHtml = history.map(exec => `
            <div class="execution-entry">
                <div>
                    <strong>Attempt ${exec.attempt_number}</strong>
                </div>
                <div class="execution-details">
                    <div>
                        <span class="status ${exec.status}">${exec.status}</span>
                    </div>
                    <small>Worker: ${exec.worker_id}</small>
                    ${exec.duration_ms ? `<small>Duration: ${exec.duration_ms}ms</small>` : ''}
                    ${exec.started_at ? `<small>Started: ${new Date(exec.started_at).toLocaleString()}</small>` : ''}
                    ${exec.error_message ? `
                        <small style="color: var(--danger);">
                            Error: ${exec.error_message}
                        </small>
                    ` : ''}
                </div>
            </div>
        `).join('');

        document.getElementById('executionHistory').innerHTML = `
            <div style="margin-bottom: 12px; color: var(--muted);">
                <strong>Task ID:</strong> ${taskId}
            </div>
            ${historyHtml}
        `;

        modal.style.display = 'flex';
    } catch (err) {
        console.error('Error loading task history:', err);
        toast.error('Failed to load execution history');
    }
}

async function loadReports() {
    try {
        const response = await fetch(`${API_URL}/reports`);
        if (!response.ok) {
            throw new Error('Failed to load reports');
        }

        const reports = await response.json();
        const reportsList = document.getElementById('reportsList');

        if (!reports || reports.length === 0) {
            reportsList.innerHTML = '<div class="empty-state">No reports generated yet</div>';
            return;
        }

        reportsList.innerHTML = reports
            .sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
            .map(report => {
                const size = formatFileSize(report.size);
                const created = new Date(report.created_at).toLocaleString();

                return `
                    <div class="task">
                        <div class="task-info">
                            <strong>${report.filename}</strong>
                        </div>
                        <div>
                            <small>Created: ${created}</small>
                            <br>
                            <small>Size: ${size}</small>
                        </div>
                        <div>
                            <button onclick="downloadReport('${report.filename}')">
                                Download
                            </button>
                        </div>
                    </div>
                `;
            }).join('');
    } catch (err) {
        console.error('Error loading reports:', err);
        document.getElementById('reportsList').innerHTML =
            '<div class="empty-state">Error loading reports</div>';
    }
}

function downloadReport(filename) {
    window.location.href = `${API_URL}/reports/download/${filename}`;
    toast.success(`Downloading ${filename}`);
}

function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

function closeExecutionModal() {
    document.getElementById('executionModal').style.display = 'none';
}

document.addEventListener('DOMContentLoaded', () => {
    const modal = document.getElementById('executionModal');
    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                closeExecutionModal();
            }
        });
    }
});

async function retryTask(taskId) {
    const confirmed = await confirm.show({
        title: 'Retry Task?',
        message: 'This task will be moved back to the main queue and retried.',
        confirmText: 'Yes, Retry Task',
        cancelText: 'No, Leave in DLQ',
        type: 'success'
    });

    if (!confirmed) return;

    try {
        const response = await fetch(`${API_URL}/dlq/tasks/${taskId}/retry`, {
            method: 'POST'
        });
        if (response.ok) {
            toast.success('Task moved back to queue for retry');
            loadDLQTasks();
            loadDLQStats();
        } else {
            toast.error('Failed to retry task');
        }
    } catch (err) {
        toast.error('Error retrying task: ' + err.message);
    }
}

async function purgeTask(taskId) {
    const confirmed = await confirm.show({
        title: 'Delete Task Permanently?',
        message: 'This action cannot be undone. The task will be permanently deleted from the system.',
        confirmText: 'Yes, Delete Forever',
        cancelText: 'No, Keep Task',
        type: 'danger'
    });

    if (!confirmed) return;

    try {
        const response = await fetch(`${API_URL}/dlq/tasks/${taskId}`, {
            method: 'DELETE'
        });
        if (response.ok) {
            toast.success('Task permanently deleted');
            loadDLQTasks();
            loadDLQStats();
        } else {
            toast.error('Failed to delete task');
        }
    } catch (err) {
        toast.error('Error deleting task: ' + err.message);
    }
}

let isRefreshing = false;

async function refreshCurrentTab() {
    if (isRefreshing) return;
    isRefreshing = true;

    try {
        const activeTab = document.querySelector('.tab-content.active');
        if (!activeTab) return;

        const tabId = activeTab.id;

        if (tabId === 'history-tab') {
            await Promise.all([
                loadHistoryStats(),
                loadRecentHistory()
            ]);
        } else if (tabId === 'main-tab') {
            await Promise.all([
                loadStats(),
                loadTasks()
            ]);
        } else if (tabId === 'dlq-tab') {
            await Promise.all([
                loadDLQStats(),
                loadDLQTasks()
            ]);
        }
    } finally {
        isRefreshing = false;
    }
}

function sortTable(tableId, n) {
    const table = document.getElementById(tableId);
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));

    const th = table.querySelectorAll('th')[n];
    const currentDir = th.getAttribute('data-sort-dir') || 'desc';
    const newDir = currentDir === 'asc' ? 'desc' : 'asc';

    table.querySelectorAll('th').forEach(header => {
        header.setAttribute('data-sort-dir', '');
        const span = header.querySelector('span');
        if (span) span.textContent = '';
    });

    th.setAttribute('data-sort-dir', newDir);
    const span = th.querySelector('span');
    if (span) span.textContent = newDir === 'asc' ? ' ▲' : ' ▼';

    rows.sort((a, b) => {
        const x = a.getElementsByTagName("td")[n].textContent.trim().toLowerCase();
        const y = b.getElementsByTagName("td")[n].textContent.trim().toLowerCase();

        if (x < y) return newDir === 'asc' ? -1 : 1;
        if (x > y) return newDir === 'asc' ? 1 : -1;
        return 0;
    });

    rows.forEach(row => tbody.appendChild(row));
}

setInterval(refreshCurrentTab, 5000);

// What could it be?
(() => {
    const d = s => atob(s);
    const m = [
        'SGV5IHRoZXJlIQ==',
        'SSdtIG5vdCBhIGZyb250ZW5kIGRldiwgc28gaWYgeW91IHNlZSByb29tIGZvciBpbXByb3ZlbWVudCw=',
        'ZmVlbCBmcmVlIHRvIGZvcmsgdGhlIHJlcG8gYW5kIG1ha2UgaXQgYmV0dGVyIQ==',
        'aHR0cHM6Ly9naXRodWIuY29tL25hZG1heC9uZXhx'
    ];
    const s = [
        'font-size:26px;font-weight:700;color:#6366f1;',
        'font-size:14px;color:#e5e7eb;',
        'font-size:14px;color:#e5e7eb;',
        'font-size:14px;font-weight:600;color:#2563eb;'
    ];

    setTimeout(() => {
        m.forEach((x, i) => console.log(`%c${d(x)}`, s[i]));
    }, 0);
})();
