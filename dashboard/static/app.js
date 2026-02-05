(function() {
    const processesBody = document.getElementById('processes-body');
    const exitedFilter = document.getElementById('exited-filter');
    const refreshBtn = document.getElementById('refresh-btn');
    const noSelection = document.getElementById('no-selection');
    const processDetail = document.getElementById('process-detail');
    const logsContent = document.getElementById('logs-content');
    const logsStatus = document.getElementById('logs-status');
    const detailKillBtn = document.getElementById('detail-kill-btn');

    let autoRefreshInterval = null;
    let currentLogStream = null;
    let streamId = 0; // Used to track which stream is current
    let selectedProcessId = null;
    let processesCache = [];

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
        if (!dateStr) return '-';
        const date = new Date(dateStr);
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return seconds + 's ago';
        if (seconds < 3600) return Math.floor(seconds / 60) + 'm ago';
        if (seconds < 86400) return Math.floor(seconds / 3600) + 'h ago';
        return Math.floor(seconds / 86400) + 'd ago';
    }

    function formatTimestamp(dateStr) {
        if (!dateStr) return '-';
        const date = new Date(dateStr);
        return date.toLocaleString();
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
            return '<span class="muted">-</span>';
        }
        return Object.entries(tags)
            .map(([k, v]) => `<span class="tag"><span class="tag-key">${escapeHtml(k)}:</span><span class="tag-value">${escapeHtml(v)}</span></span>`)
            .join('');
    }

    function formatTagsCompact(tags) {
        if (!tags || Object.keys(tags).length === 0) {
            return '';
        }
        return Object.entries(tags)
            .map(([k, v]) => `<span class="tag"><span class="tag-key">${escapeHtml(k)}:</span><span class="tag-value">${escapeHtml(v)}</span></span>`)
            .join('');
    }

    function formatPorts(ports) {
        if (!ports || ports.length === 0) {
            return '<span class="muted">-</span>';
        }
        return `<span class="ports">${ports.join(', ')}</span>`;
    }

    function formatEnv(env) {
        if (!env || Object.keys(env).length === 0) {
            return '<span class="muted">-</span>';
        }
        return Object.entries(env)
            .map(([k, v]) => `<span class="env-var"><span class="env-key">${escapeHtml(k)}=</span><span class="env-value">${escapeHtml(v)}</span></span>`)
            .join('');
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

    function renderProcessList(processes) {
        if (processes === null) {
            processesBody.innerHTML = '<div class="loading">Error loading processes</div>';
            return;
        }

        if (processes.length === 0) {
            processesBody.innerHTML = '<div class="empty-message">No processes found</div>';
            return;
        }

        // Sort: running first, then by start time (newest first)
        processes.sort((a, b) => {
            if (a.status === 'running' && b.status !== 'running') return -1;
            if (a.status !== 'running' && b.status === 'running') return 1;
            return new Date(b.started_at) - new Date(a.started_at);
        });

        processesCache = processes;

        processesBody.innerHTML = processes.map(proc => `
            <div class="process-item ${selectedProcessId === proc.id ? 'selected' : ''}"
                 data-id="${escapeHtml(proc.id)}"
                 data-status="${proc.status}"
                 onclick="window.selectProcess('${proc.id}')">
                <div class="process-item-header">
                    <span class="status status-${proc.status}">${proc.status}</span>
                    <span class="process-time">${formatTimeAgo(proc.started_at)}</span>
                </div>
                <div class="process-command">${escapeHtml(formatCommand(proc.command, proc.args))}</div>
                <div class="process-meta">
                    ${proc.exited_at ? `<span class="exit-info">exited ${formatTimeAgo(proc.exited_at)}</span>` : ''}
                </div>
                <div class="process-tags">${formatTagsCompact(proc.tags)}</div>
            </div>
        `).join('');
    }

    function showProcessDetail(proc) {
        if (!proc) {
            noSelection.classList.remove('hidden');
            processDetail.classList.add('hidden');
            return;
        }

        noSelection.classList.add('hidden');
        processDetail.classList.remove('hidden');

        document.getElementById('detail-command').textContent = formatCommand(proc.command, proc.args);
        document.getElementById('detail-status').textContent = proc.status;
        document.getElementById('detail-status').className = `status status-${proc.status}`;
        document.getElementById('detail-id').textContent = proc.id;
        document.getElementById('detail-pid').textContent = proc.pid;
        document.getElementById('detail-started').textContent = formatTimestamp(proc.started_at);
        document.getElementById('detail-exited').textContent = proc.exited_at ? formatTimestamp(proc.exited_at) : '-';
        document.getElementById('detail-cwd').textContent = proc.cwd || '-';
        document.getElementById('detail-ports').innerHTML = formatPorts(proc.ports);
        document.getElementById('detail-tags').innerHTML = formatTags(proc.tags);
        document.getElementById('detail-env').innerHTML = formatEnv(proc.env);

        detailKillBtn.disabled = proc.status !== 'running';
    }

    function closeLogStream() {
        if (currentLogStream) {
            currentLogStream.close();
            currentLogStream = null;
        }
    }

    function startLogStream(processId) {
        // Close any existing stream
        closeLogStream();

        // Increment stream ID to track this specific stream
        streamId++;
        const thisStreamId = streamId;

        logsContent.textContent = 'Connecting...';
        setLogsStatus('');

        const stream = new EventSource(`/api/processes/${processId}/logs/stream`);
        currentLogStream = stream;

        let hasContent = false;
        let pendingText = '';
        let updateScheduled = false;

        function flushPendingText() {
            if (pendingText && streamId === thisStreamId) {
                if (!hasContent) {
                    logsContent.textContent = '';
                    hasContent = true;
                }
                logsContent.textContent += pendingText;
                logsContent.scrollTop = logsContent.scrollHeight;
                pendingText = '';
            }
            updateScheduled = false;
        }

        stream.onopen = function() {
            // Only update if this is still the current stream
            if (streamId === thisStreamId) {
                setLogsStatus('streaming');
            }
        };

        stream.onmessage = function(event) {
            // Only update if this is still the current stream
            if (streamId !== thisStreamId) {
                stream.close();
                return;
            }
            // Batch updates to avoid overwhelming the DOM
            pendingText += event.data + '\n';
            if (!updateScheduled) {
                updateScheduled = true;
                requestAnimationFrame(flushPendingText);
            }
        };

        stream.onerror = function() {
            // Only handle error if this is still the current stream
            if (streamId !== thisStreamId) {
                return;
            }
            // Flush any remaining text
            flushPendingText();
            if (!hasContent) {
                logsContent.textContent = '(no output or connection error)';
            }
            setLogsStatus('disconnected');
            currentLogStream = null;
        };
    }

    window.selectProcess = function(processId) {
        selectedProcessId = processId;

        // Update selection in list
        document.querySelectorAll('.process-item').forEach(item => {
            item.classList.toggle('selected', item.dataset.id === processId);
        });

        // Find the process in cache
        const proc = processesCache.find(p => p.id === processId);
        showProcessDetail(proc);

        // Start streaming logs
        if (proc) {
            startLogStream(processId);
        }
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
            // Refresh the page to show new status
            window.location.reload();
        } catch (error) {
            alert('Error killing process: ' + error.message);
        }
    };

    detailKillBtn.addEventListener('click', function() {
        if (selectedProcessId) {
            window.killProcess(selectedProcessId);
        }
    });

    async function refresh() {
        const processes = await fetchProcesses();
        renderProcessList(processes);

        // Update detail view if a process is selected
        if (selectedProcessId) {
            const proc = processes?.find(p => p.id === selectedProcessId);
            if (proc) {
                showProcessDetail(proc);
            }
        }
    }

    exitedFilter.addEventListener('change', refresh);
    refreshBtn.addEventListener('click', refresh);

    // Initial load and auto-refresh every 5 seconds
    refresh();
    autoRefreshInterval = setInterval(refresh, 5000);
})();
