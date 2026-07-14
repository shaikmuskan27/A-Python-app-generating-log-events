const API_URL = '/api'; // Because of Nginx reverse proxy

const elements = {
    totalLogs: document.getElementById('totalLogs'),
    errorLogs: document.getElementById('errorLogs'),
    infoLogs: document.getElementById('infoLogs'),
    logsBody: document.getElementById('logsBody'),
    severityFilter: document.getElementById('severityFilter'),
    refreshBtn: document.getElementById('refreshBtn'),
    loading: document.getElementById('loading')
};

function formatTimestamp(isoString) {
    const date = new Date(isoString);
    return date.toLocaleString();
}

function createLogBadge(severity) {
    const className = severity === 'ERROR' ? 'badge-error' : 'badge-info';
    return `<span class="badge ${className}">${severity}</span>`;
}

async function fetchStats() {
    try {
        const response = await fetch(`${API_URL}/stats`);
        if (!response.ok) throw new Error('Failed to fetch stats');
        const data = await response.json();
        
        // Animate numbers
        animateValue(elements.totalLogs, parseInt(elements.totalLogs.innerText) || 0, data.total_logs, 1000);
        animateValue(elements.errorLogs, parseInt(elements.errorLogs.innerText) || 0, data.error_logs, 1000);
        animateValue(elements.infoLogs, parseInt(elements.infoLogs.innerText) || 0, data.info_logs, 1000);
    } catch (err) {
        console.error(err);
        if (elements.totalLogs.innerText === '--') {
            elements.totalLogs.innerText = 'Err';
        }
    }
}

async function fetchLogs() {
    elements.loading.style.display = 'block';
    
    try {
        const severity = elements.severityFilter.value;
        const query = severity !== 'ALL' ? `?severity=${severity}` : '';
        const response = await fetch(`${API_URL}/logs${query}`);
        if (!response.ok) throw new Error('Failed to fetch logs');
        
        const logs = await response.json();
        elements.logsBody.innerHTML = '';
        
        if (logs.length === 0) {
            elements.logsBody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--text-muted); padding: 2rem;">No logs found.</td></tr>`;
            return;
        }

        logs.forEach((log, index) => {
            const tr = document.createElement('tr');
            tr.className = 'log-row';
            tr.style.animationDelay = `${Math.min(index * 0.03, 1.5)}s`; // Staggered animation capped
            
            tr.innerHTML = `
                <td style="white-space: nowrap; color: var(--text-muted);">${formatTimestamp(log.timestamp)}</td>
                <td>${createLogBadge(log.severity)}</td>
                <td class="monospace">${log.service_name}</td>
                <td class="monospace">${log.container_id.substring(0, 12)}</td>
                <td>${log.message}</td>
            `;
            elements.logsBody.appendChild(tr);
        });
    } catch (err) {
        console.error(err);
        elements.logsBody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: var(--accent-red); padding: 2rem;">Failed to load logs. Is the API running?</td></tr>`;
    } finally {
        elements.loading.style.display = 'none';
    }
}

// Simple number counter animation
function animateValue(obj, start, end, duration) {
    if (start === end) return;
    let startTimestamp = null;
    const step = (timestamp) => {
        if (!startTimestamp) startTimestamp = timestamp;
        const progress = Math.min((timestamp - startTimestamp) / duration, 1);
        obj.innerHTML = Math.floor(progress * (end - start) + start).toLocaleString();
        if (progress < 1) {
            window.requestAnimationFrame(step);
        }
    };
    window.requestAnimationFrame(step);
}

// Event Listeners
elements.refreshBtn.addEventListener('click', () => {
    // Spin icon
    const svg = elements.refreshBtn.querySelector('svg');
    svg.style.transition = 'transform 0.5s ease';
    svg.style.transform = `rotate(360deg)`;
    setTimeout(() => { svg.style.transition = 'none'; svg.style.transform = `rotate(0deg)`; }, 500);
    
    fetchStats();
    fetchLogs();
});

elements.severityFilter.addEventListener('change', fetchLogs);

// Initial Load
fetchStats();
fetchLogs();

// Auto refresh every 10 seconds
setInterval(() => {
    fetchStats();
    fetchLogs();
}, 10000);
