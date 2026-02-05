(function() {
    const processesBody = document.getElementById('processes-body');
    const exitedFilter = document.getElementById('exited-filter');
    const refreshBtn = document.getElementById('refresh-btn');
    const logsModal = document.getElementById('logs-modal');
    const logsProcessId = document.getElementById('logs-process-id');
    const logsContent = document.getElementById('logs-content');
    const logsStatus = document.getElementById('logs-status');
    const closeLogs = document.getElementById('close-logs');

    let autoRefreshInterval = null;
    let currentLogStream = null;

    function setLogsStatus(status) {
        logsStatus.className = 'logs-status';
        if (status === 'streaming') {
            logsStatus.textContent = 'LIVE';
            logsStatus.classList.add('streaming');
        } else if (status === 'disconnected') {
            logsStatus.textContent = 'ENDED';
            logsStatus.classList.add('disconnected');
        } else {
            logsStatus.textContent = '';
        }
    }

    function formatTimeAgo(dateStr) {
        const date = new Date(dateStr);
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return seconds + 's ago';
        if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
        if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
        return Math.floor(seconds / 86400) + 'd ago';
    }

    function formatCommand(command, args) {
        let full = command;
        if (args && args.length > 0) {
            full += ' ' + args.join(' ');
        }
        return full;
    }

    function formatTags(tags) {
        if (!tags || Object.keys(tags).length === 0) {
            return '-';
        }
        return Object.entries(tags)
            .map(([k, v]) => `<span class="tag"><span class="tag-key">${escapeHtml(k)}:</span><span class="tag-value">${escapeHtml(v)}</span></span>`)
            .join('');
    }

    function formatPorts(ports) {
        if (!ports || ports.length === 0) {
            return '-';
        }
        return ports.join(', ');
    }

    function formatCwd(cwd) {
        if (!cwd) {
            return '<span class="muted">-</span>';
        }
        // Show last 2 path components for brevity
        const parts = cwd.split('/').filter(p => p);
        const short = parts.length > 2
            ? '.../' + parts.slice(-2).join('/')
            : cwd;
        return `<span class="cwd" title="${escapeHtml(cwd)}">${escapeHtml(short)}</span>`;
    }

    function formatEnv(env) {
        if (!env || Object.keys(env).length === 0) {
            return '<span class="muted">-</span>';
        }
        const entries = Object.entries(env);
        const html = entries
            .map(([k, v]) => `<span class="env-var"><span class="env-key">${escapeHtml(k)}=</span><span class="env-value">${escapeHtml(v)}</span></span>`)
            .join('');
        return `<div class="env-vars">${html}</div>`;
    }

    function escapeHtml(str) {
        if (str == null) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    async function fetchProcesses() {
        const exitedSecs = exitedFilter.value;
        const url = exitedSecs === '0'
            ? '/api/processes?exited_since_secs=999999999'
            : `/api/processes?exited_since_secs=${exitedSecs}`;

        try {
            const response = await fetch(url);
            if (!response.ok) {
                throw new Error('Failed to fetch processes');
            }
            return await response.json();
        } catch (error) {
            console.error('Error fetching processes:', error);
            return null;
        }
    }

    function renderProcesses(processes) {
        if (processes === null) {
            processesBody.innerHTML = '<tr><td colspan="10" class="loading">Error loading processes</td></tr>';
            return;
        }

        if (processes.length === 0) {
            processesBody.innerHTML = '<tr><td colspan="10" class="empty-message">No processes found</td></tr>';
            return;
        }

        // Sort: running first, then by start time (newest first)
        processes.sort((a, b) => {
            if (a.status === 'running' && b.status !== 'running') return -1;
            if (a.status !== 'running' && b.status === 'running') return 1;
            return new Date(b.started_at) - new Date(a.started_at);
        });

        processesBody.innerHTML = processes.map(proc => `
            <tr class="process-row" data-status="${proc.status}">
                <td><code>${escapeHtml(proc.id)}</code></td>
                <td class="command" title="${escapeHtml(formatCommand(proc.command, proc.args))}">${escapeHtml(formatCommand(proc.command, proc.args))}</td>
                <td class="cwd-cell">${formatCwd(proc.cwd)}</td>
                <td><span class="status status-${proc.status}">${proc.status}</span></td>
                <td>${proc.pid}</td>
                <td class="ports">${formatPorts(proc.ports)}</td>
                <td class="env-cell">${formatEnv(proc.env)}</td>
                <td class="tags">${formatTags(proc.tags)}</td>
                <td class="time-ago" title="${proc.started_at}">${formatTimeAgo(proc.started_at)}</td>
                <td class="actions">
                    <button class="btn-logs" onclick="window.showLogs('${proc.id}')">Logs</button>
                    <button class="btn-kill" onclick="window.killProcess('${proc.id}')" ${proc.status !== 'running' ? 'disabled' : ''}>Kill</button>
                </td>
            </tr>
        `).join('');
    }

    async function refresh() {
        const processes = await fetchProcesses();
        renderProcesses(processes);
    }

    function closeLogStream() {
        if (currentLogStream) {
            currentLogStream.close();
            currentLogStream = null;
        }
    }

    window.showLogs = function(processId) {
        // Close any existing stream
        closeLogStream();

        logsProcessId.textContent = processId;
        logsContent.textContent = 'Connecting...';
        setLogsStatus('');
        logsModal.classList.remove('hidden');

        // Use EventSource for streaming logs
        currentLogStream = new EventSource(`/api/processes/${processId}/logs/stream`);

        let hasContent = false;

        currentLogStream.onopen = function() {
            setLogsStatus('streaming');
        };

        currentLogStream.onmessage = function(event) {
            if (!hasContent) {
                logsContent.textContent = '';
                hasContent = true;
            }
            logsContent.textContent += event.data + '\n';
            logsContent.scrollTop = logsContent.scrollHeight;
        };

        currentLogStream.onerror = function(event) {
            if (!hasContent) {
                logsContent.textContent = '(no output or connection error)';
            }
            setLogsStatus('disconnected');
            closeLogStream();
        };
    };

    window.killProcess = async function(processId) {
        if (!confirm(`Kill process ${processId}?`)) {
            return;
        }

        try {
            const response = await fetch(`/api/processes/${processId}/kill`, {
                method: 'POST'
            });
            if (!response.ok) {
                throw new Error('Failed to kill process');
            }
            await refresh();
        } catch (error) {
            alert('Error killing process: ' + error.message);
        }
    };

    function hideLogsModal() {
        closeLogStream();
        logsModal.classList.add('hidden');
    }

    closeLogs.addEventListener('click', hideLogsModal);

    logsModal.addEventListener('click', (e) => {
        if (e.target === logsModal) {
            hideLogsModal();
        }
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && !logsModal.classList.contains('hidden')) {
            hideLogsModal();
        }
    });

    exitedFilter.addEventListener('change', refresh);
    refreshBtn.addEventListener('click', refresh);

    // Initial load and auto-refresh every 5 seconds
    refresh();
    autoRefreshInterval = setInterval(refresh, 5000);
})();
