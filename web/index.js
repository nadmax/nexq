const API_URL = '/api';
const exampleCode = [
    { to: "user@example.com", subject: "Hello from Nexq", body: "This is a custom email!" },
    { report_type: "monthly", schedule_in: 3600 },
    { image_url: "https://example.com/image.jpg", operations: ["resize", "compress"] }
];
let editor;
let exampleEditors = [];

require.config({ paths: { vs: 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.53.0/min/vs' } });
require(['vs/editor/editor.main'], function () {
    editor = monaco.editor.create(document.getElementById('editor'), {
        value: JSON.stringify({
            to: "user@example.com",
            subject: "Hello from Nexq",
            body: "This is a custom email!"
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
    exampleCode.forEach((code, i) => {
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
        exampleEditors.push(exampleEditor);
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
    }
}

function copyCode(button, index) {
    const code = exampleEditors[index].getValue();

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
        loadTasks();
    } catch (err) {
        alert('Error creating task: ' + err.message);
    }
});

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
        if (!tasks || tasks.length === 0) {
            dlqList.innerHTML = '<div class="empty-state">✅ No failed tasks in the dead letter queue</div>';
            return;
        }

        dlqList.innerHTML = tasks.map(task => `
                    <div class="dlq-task">
                        <div class="dlq-info">
                            <div>
                                <strong>${task.type}</strong>
                                <span class="priority ${getPriorityLabel(task.priority)}">${getPriorityLabel(task.priority).toUpperCase()}</span>
                                <br>
                                <small>${task.id}</small>
                            </div>
                            <div>
                                <small>Failed: ${new Date(task.moved_to_dlq_at).toLocaleString()}</small><br>
                                <small>Retries: ${task.retry_count}/${task.max_retries}</small>
                            </div>
                        </div>
                        ${task.failure_reason ? `
                            <div class="failure-reason">
                                <strong>Failure Reason:</strong> ${task.failure_reason}
                            </div>
                        ` : ''}
                        <div class="dlq-actions">
                            <button class="success" onclick="retryTask('${task.id}')">Retry Task</button>
                            <button class="danger" onclick="purgeTask('${task.id}')">Delete Permanently</button>
                        </div>
                    </div>
                `).join('');
    } catch (err) {
        console.error('Error loading DLQ tasks:', err);
        document.getElementById('dlqList').innerHTML = '<div class="empty-state">Error loading DLQ tasks</div>';
    }
}

async function loadDLQStats() {
    try {
        const response = await fetch(`${API_URL}/dlq/stats`);
        if (!response.ok) {
            const err = await response.json();
            throw Error(err.error || 'failed to load dead letter stats');
        }

        const stats = await response.json();
        document.getElementById('dlqStats').innerHTML = `
                    <div class="stats-grid">
                        <div class="stat-card">
                            <h3>Total Failed</h3>
                            <p class="stat-number">${stats.total_tasks || 0}</p>
                        </div>
                        ${stats.oldest_task_time ? `
                            <div class="stat-card">
                                <h3>Oldest Task</h3>
                                <p style="font-size: 14px; margin-top: 10px;">${new Date(stats.oldest_task_time).toLocaleString()}</p>
                            </div>
                        ` : ''}
                        ${stats.newest_task_time ? `
                            <div class="stat-card">
                                <h3>Newest Task</h3>
                                <p style="font-size: 14px; margin-top: 10px;">${new Date(stats.newest_task_time).toLocaleString()}</p>
                            </div>
                        ` : ''}
                    </div>
                `;
    } catch (err) {
        console.error('Error loading DLQ stats:', err);
    }
}

async function retryTask(taskId) {
    if (!confirm('Retry this task? It will be moved back to the main queue.')) {
        return;
    }

    try {
        const response = await fetch(`${API_URL}/dlq/tasks/${taskId}/retry`, {
            method: 'POST'
        });
        if (response.ok) {
            alert('Task moved back to queue for retry');
            loadDLQTasks();
            loadDLQStats();
        } else {
            alert('Failed to retry task');
        }
    } catch (err) {
        alert('Error retrying task: ' + err.message);
    }
}

async function purgeTask(taskId) {
    if (!confirm('Permanently delete this task? This cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch(`${API_URL}/dlq/tasks/${taskId}`, {
            method: 'DELETE'
        });
        if (response.ok) {
            alert('Task permanently deleted');
            loadDLQTasks();
            loadDLQStats();
        } else {
            alert('Failed to delete task');
        }
    } catch (err) {
        alert('Error deleting task: ' + err.message);
    }
}

loadStats();
loadTasks();
setInterval(() => {
    loadStats();
    loadTasks();
}, 5000);

function sortTable(tableId, n) {
    const table = document.getElementById(tableId);
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));

    const th = table.querySelectorAll('th')[n];
    const currentDir = th.getAttribute('data-sort-dir') || 'desc'; // Default to desc so first click becomes asc
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
