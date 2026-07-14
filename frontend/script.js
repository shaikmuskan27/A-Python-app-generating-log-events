const API_URL = '/api';

const elements = {
    totalLogs: document.getElementById('totalLogs'),
    errorLogs: document.getElementById('errorLogs'),
    infoLogs: document.getElementById('infoLogs'),
    logsBody: document.getElementById('logsBody'),
    severityFilter: document.getElementById('severityFilter'),
    searchInput: document.getElementById('searchInput'),
    refreshBtn: document.getElementById('refreshBtn'),
    loading: document.getElementById('loading'),
    toastContainer: document.getElementById('toastContainer')
};

// Chart Instances
let volumeChart = null;
let severityChart = null;

// State
let lastErrorCount = 0;

Chart.defaults.color = '#94a3b8';
Chart.defaults.font.family = "'Outfit', sans-serif";

function initCharts() {
    const volCtx = document.getElementById('volumeChart').getContext('2d');
    volumeChart = new Chart(volCtx, {
        type: 'bar',
        data: { labels: [], datasets: [{ label: 'Logs', data: [], backgroundColor: 'rgba(59, 130, 246, 0.6)', borderRadius: 4 }] },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: { grid: { color: 'rgba(255,255,255,0.05)' } },
                x: { grid: { display: false } }
            },
            plugins: { legend: { display: false } }
        }
    });

    const sevCtx = document.getElementById('severityChart').getContext('2d');
    severityChart = new Chart(sevCtx, {
        type: 'doughnut',
        data: {
            labels: ['ERROR', 'INFO'],
            datasets: [{
                data: [0, 0],
                backgroundColor: ['rgba(239, 68, 68, 0.8)', 'rgba(16, 185, 129, 0.8)'],
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '70%',
            plugins: {
                legend: { position: 'bottom' }
            }
        }
    });
}

function showToast(message) {
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.innerHTML = `<i>⚠️</i> <div><strong>New Alert</strong><br/>${message}</div>`;
    elements.toastContainer.appendChild(toast);
    
    setTimeout(() => {
        toast.classList.add('fade-out');
        setTimeout(() => toast.remove(), 500);
    }, 5000);
}

async function fetchStats() {
    try {
        const response = await fetch(`${API_URL}/stats`);
        if (!response.ok) return;
        const data = await response.json();
        
        animateValue(elements.totalLogs, parseInt(elements.totalLogs.innerText) || 0, data.total_logs, 1000);
        animateValue(elements.errorLogs, parseInt(elements.errorLogs.innerText) || 0, data.error_logs, 1000);
        animateValue(elements.infoLogs, parseInt(elements.infoLogs.innerText) || 0, data.info_logs, 1000);

        // Update Severity Chart
        severityChart.data.datasets[0].data = [data.error_logs, data.info_logs];
        severityChart.update();

        // Check for new errors
        if (lastErrorCount !== 0 && data.error_logs > lastErrorCount) {
            showToast(`${data.error_logs - lastErrorCount} new error(s) detected!`);
        }
        lastErrorCount = data.error_logs;

    } catch (err) {
        console.error('Stats Error:', err);
    }
}

async function fetchLogs() {
    elements.loading.style.display = 'block';
    
    try {
        const severity = elements.severityFilter.value;
        const search = elements.searchInput.value.trim();
        
        let query = `?severity=${severity}`;
        if (search) query += `&search=${encodeURIComponent(search)}`;
        
        const response = await fetch(`${API_URL}/logs${query}`);
        if (!response.ok) throw new Error('API Error');
        const logs = await response.json();
        
        elements.logsBody.innerHTML = '';
        if (logs.length === 0) {
            elements.logsBody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 3rem; color: var(--text-muted);">No logs match your criteria.</td></tr>`;
            updateVolumeChart([]);
            return;
        }

        updateVolumeChart(logs);

        logs.forEach((log, index) => {
            const tr = document.createElement('tr');
            tr.className = 'log-row';
            tr.style.animationDelay = `${Math.min(index * 0.02, 1)}s`;
            
            const badgeClass = log.severity === 'ERROR' ? 'badge-error' : 'badge-info';
            const date = new Date(log.timestamp);
            const timeStr = `${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}:${date.getSeconds().toString().padStart(2,'0')}`;

            tr.innerHTML = `
                <td style="white-space: nowrap; color: var(--text-muted);">${timeStr}</td>
                <td><span class="badge ${badgeClass}">${log.severity}</span></td>
                <td class="monospace">${log.service_name}</td>
                <td class="monospace">${log.container_id.substring(0, 8)}</td>
                <td>${escapeHtml(log.message)}</td>
            `;
            elements.logsBody.appendChild(tr);
        });
    } catch (err) {
        console.error(err);
    } finally {
        elements.loading.style.display = 'none';
    }
}

function updateVolumeChart(logs) {
    // Extract times (HH:MM) and count them
    const counts = {};
    logs.forEach(l => {
        const d = new Date(l.timestamp);
        const key = `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}`;
        counts[key] = (counts[key] || 0) + 1;
    });

    // Sort keys chronologically
    const labels = Object.keys(counts).sort();
    const data = labels.map(k => counts[k]);

    // Keep last 15 minutes max
    volumeChart.data.labels = labels.slice(-15);
    volumeChart.data.datasets[0].data = data.slice(-15);
    volumeChart.update();
}

function escapeHtml(unsafe) {
    return unsafe
         .replace(/&/g, "&amp;")
         .replace(/</g, "&lt;")
         .replace(/>/g, "&gt;");
}

function animateValue(obj, start, end, duration) {
    if (start === end) return;
    let startTimestamp = null;
    const step = (timestamp) => {
        if (!startTimestamp) startTimestamp = timestamp;
        const progress = Math.min((timestamp - startTimestamp) / duration, 1);
        obj.innerHTML = Math.floor(progress * (end - start) + start).toLocaleString();
        if (progress < 1) window.requestAnimationFrame(step);
    };
    window.requestAnimationFrame(step);
}

// Debounce for search input
let searchTimeout;
elements.searchInput.addEventListener('input', () => {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(fetchLogs, 500);
});

elements.severityFilter.addEventListener('change', fetchLogs);

elements.refreshBtn.addEventListener('click', () => {
    const icon = elements.refreshBtn.querySelector('i');
    icon.style.display = 'inline-block';
    icon.style.transition = 'transform 0.5s ease';
    icon.style.transform = 'rotate(360deg)';
    setTimeout(() => { icon.style.transition = 'none'; icon.style.transform = 'rotate(0deg)'; }, 500);
    fetchStats();
    fetchLogs();
});

// Initialization
initCharts();
fetchStats();
fetchLogs();

// Auto refresh
setInterval(() => {
    fetchStats();
    // Only auto-refresh logs if not searching to prevent losing place
    if (!elements.searchInput.value) {
        fetchLogs();
    }
}, 5000);
