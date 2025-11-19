// ===== GLOBAL STATE =====
let ws = null;
let autoScroll = true;
let dataHistory = [];
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;

// Pagination & Filter state
let currentPage = 1;
let itemsPerPage = 50;
let totalRecords = 0;
let currentFilter = {
    startDate: null,
    endDate: null,
    timeRange: '1h' // Default 1 hour
};

// ===== INITIALIZATION =====
window.addEventListener('DOMContentLoaded', () => {
    const user = checkAuth();
    if (!user) {
        window.location.href = '/view/login.html';
        return;
    }
    
    document.getElementById('username').textContent = user.username || 'Admin';
    
    addConsoleLog('🚀 Dashboard initialized', 'success');
    
    // Set default time range (last 1 hour)
    setDefaultTimeRange();
    
    // Fetch dengan filter default
    fetchHistoricalDataWithFilter();
    
    // Init WebSocket untuk real-time
    initWebSocket();
    
    setInterval(updateLastUpdateTime, 1000);
});

// ===== SET DEFAULT TIME RANGE =====
function setDefaultTimeRange() {
    const now = new Date();
    const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);
    
    currentFilter.startDate = oneHourAgo;
    currentFilter.endDate = now;
    
    // Update UI inputs jika ada
    const startInput = document.getElementById('filterStartDate');
    const endInput = document.getElementById('filterEndDate');
    
    if (startInput) {
        startInput.value = formatDateTimeLocal(oneHourAgo);
    }
    if (endInput) {
        endInput.value = formatDateTimeLocal(now);
    }
}

function formatDateTimeLocal(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    
    return `${year}-${month}-${day}T${hours}:${minutes}`;
}

// ===== FETCH DATA DENGAN FILTER =====
async function fetchHistoricalDataWithFilter() {
    addConsoleLog('📥 Fetching filtered data from IoTDB...', 'info');
    
    try {
        const token = getToken();
        
        if (!token) {
            addConsoleLog('⚠️ No authentication token found', 'warning');
            return;
        }
        
        const startTime = currentFilter.startDate.getTime();
        const endTime = currentFilter.endDate.getTime();
        
        addConsoleLog(`🔍 Filter: ${currentFilter.startDate.toLocaleString()} to ${currentFilter.endDate.toLocaleString()}`, 'info');
        
        // Gunakan endpoint history dengan time range
        const url = `/api/energy/history?device_id=ESP32_PZEM&start_time=${startTime}&end_time=${endTime}&limit=10000`;
        
        const response = await fetch(url, {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            }
        });
        
        if (!response.ok) {
            const errorText = await response.text();
            addConsoleLog(`❌ Failed to fetch: ${errorText}`, 'error');
            showNoDataMessage('Failed to load data from IoTDB');
            return;
        }
        
        const result = await response.json();
        
        let dataArray = [];
        
        if (result.data && Array.isArray(result.data)) {
            dataArray = result.data;
        } else if (Array.isArray(result)) {
            dataArray = result;
        }
        
        if (dataArray.length === 0) {
            addConsoleLog('⚠️ No data in selected time range', 'warning');
            showNoDataMessage('No data in selected time range');
            return;
        }
        
        addConsoleLog(`✅ Loaded ${dataArray.length} records`, 'success');
        
        // Process data
        dataHistory = [];
        totalRecords = 0;
        
        dataArray.forEach((item) => {
            if (processHistoricalDataItem(item)) {
                totalRecords++;
            }
        });
        
        if (dataHistory.length === 0) {
            addConsoleLog('⚠️ No valid data after processing', 'warning');
            showNoDataMessage('All records invalid');
            return;
        }
        
        // Sort by timestamp DESC (newest first)
        dataHistory.sort((a, b) => b.timestamp - a.timestamp);
        
        // Update dashboard dengan data terbaru
        if (dataHistory.length > 0) {
            const latestData = dataHistory[0];
            updateDashboardWithDataDirect(latestData);
            addConsoleLog(`📊 Updated with latest: ${new Date(latestData.timestamp).toLocaleString()}`, 'success');
        }
        
        // Reset to page 1 dan update table
        currentPage = 1;
        updateDataTable();
        updatePaginationControls();
        
        addConsoleLog(`📊 Summary: ${dataHistory.length} records in ${Math.ceil(dataHistory.length / itemsPerPage)} pages`, 'success');
        
    } catch (error) {
        console.error('❌ Fetch error:', error);
        addConsoleLog(`❌ Failed: ${error.message}`, 'error');
        showNoDataMessage(`Error: ${error.message}`);
    }
}

function processHistoricalDataItem(item) {
    // Support berbagai format timestamp
    let timestamp;
    
    if (item.timestamp) {
        if (typeof item.timestamp === 'number') {
            timestamp = item.timestamp;
        } else if (item.timestamp.UnixMilli) {
            timestamp = item.timestamp.UnixMilli();
        } else {
            timestamp = new Date(item.timestamp).getTime();
        }
    } else {
        return false;
    }
    
    if (isNaN(timestamp) || timestamp === 0) {
        return false;
    }
    
    const voltage = parseFloat(item.voltage || 0);
    const current = parseFloat(item.current || 0);
    const power = parseFloat(item.power || 0);
    const energy = parseFloat(item.energy || 0);
    const frequency = parseFloat(item.frequency || 50);
    const pf = parseFloat(item.power_factor || 1);
    
    // Skip if all values are zero
    if (voltage === 0 && current === 0 && power === 0 && energy === 0) {
        return false;
    }
    
    dataHistory.push({
        timestamp,
        voltage: isNaN(voltage) ? 0 : voltage,
        current: isNaN(current) ? 0 : current,
        power: isNaN(power) ? 0 : power,
        energy: isNaN(energy) ? 0 : energy,
        frequency: isNaN(frequency) ? 50 : frequency,
        pf: isNaN(pf) ? 1 : pf
    });
    
    return true;
}

function showNoDataMessage(message) {
    const tbody = document.getElementById('dataTableBody');
    if (tbody) {
        tbody.innerHTML = `<tr><td colspan="7" class="no-data">${message}</td></tr>`;
    }
    document.getElementById('historyCount').textContent = '0';
    document.getElementById('totalData').textContent = '0';
    document.getElementById('totalDataTable').textContent = '0';
}

// ===== QUICK FILTER FUNCTIONS =====
function applyQuickFilter(range) {
    const now = new Date();
    let startDate;
    
    switch(range) {
        case '15m':
            startDate = new Date(now.getTime() - 15 * 60 * 1000);
            break;
        case '30m':
            startDate = new Date(now.getTime() - 30 * 60 * 1000);
            break;
        case '1h':
            startDate = new Date(now.getTime() - 60 * 60 * 1000);
            break;
        case '3h':
            startDate = new Date(now.getTime() - 3 * 60 * 60 * 1000);
            break;
        case '6h':
            startDate = new Date(now.getTime() - 6 * 60 * 60 * 1000);
            break;
        case '12h':
            startDate = new Date(now.getTime() - 12 * 60 * 60 * 1000);
            break;
        case '24h':
            startDate = new Date(now.getTime() - 24 * 60 * 60 * 1000);
            break;
        case '7d':
            startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
            break;
        default:
            startDate = new Date(now.getTime() - 60 * 60 * 1000);
    }
    
    currentFilter.startDate = startDate;
    currentFilter.endDate = now;
    currentFilter.timeRange = range;
    
    // Update UI
    const startInput = document.getElementById('filterStartDate');
    const endInput = document.getElementById('filterEndDate');
    
    if (startInput) startInput.value = formatDateTimeLocal(startDate);
    if (endInput) endInput.value = formatDateTimeLocal(now);
    
    // Highlight active button
    document.querySelectorAll('.quick-filter-btn').forEach(btn => {
        btn.classList.remove('active');
        if (btn.dataset.range === range) {
            btn.classList.add('active');
        }
    });
    
    addConsoleLog(`📅 Quick filter: ${range}`, 'info');
    fetchHistoricalDataWithFilter();
}

function applyCustomFilter() {
    const startInput = document.getElementById('filterStartDate');
    const endInput = document.getElementById('filterEndDate');
    
    if (!startInput || !endInput) {
        addConsoleLog('⚠️ Filter inputs not found', 'warning');
        return;
    }
    
    const startDate = new Date(startInput.value);
    const endDate = new Date(endInput.value);
    
    if (isNaN(startDate.getTime()) || isNaN(endDate.getTime())) {
        addConsoleLog('⚠️ Invalid date range', 'warning');
        alert('Please select valid start and end dates');
        return;
    }
    
    if (startDate > endDate) {
        addConsoleLog('⚠️ Start date must be before end date', 'warning');
        alert('Start date must be before end date');
        return;
    }
    
    currentFilter.startDate = startDate;
    currentFilter.endDate = endDate;
    currentFilter.timeRange = 'custom';
    
    // Remove active class from quick filter buttons
    document.querySelectorAll('.quick-filter-btn').forEach(btn => {
        btn.classList.remove('active');
    });
    
    addConsoleLog(`📅 Custom filter applied`, 'info');
    fetchHistoricalDataWithFilter();
}

// ===== WEBSOCKET =====
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    addConsoleLog('🔌 Connecting to: ' + wsUrl, 'info');
    
    try {
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            addConsoleLog('✅ WebSocket connected', 'success');
            updateConnectionStatus(true);
            reconnectAttempts = 0;
        };
        
        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketData(data);
            } catch (error) {
                console.error('❌ Parse error:', error);
            }
        };
        
        ws.onerror = function() {
            addConsoleLog('❌ WebSocket error', 'error');
            updateConnectionStatus(false);
        };
        
        ws.onclose = function(event) {
            addConsoleLog('🔌 Disconnected (Code: ' + event.code + ')', 'warning');
            updateConnectionStatus(false);
            
            if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000);
                addConsoleLog(`🔄 Reconnecting in ${delay/1000}s...`, 'info');
                setTimeout(initWebSocket, delay);
            }
        };
        
    } catch (error) {
        addConsoleLog('❌ WebSocket creation failed: ' + error.message, 'error');
        updateConnectionStatus(false);
    }
}

function updateConnectionStatus(connected) {
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    
    if (statusDot) {
        if (connected) {
            statusDot.classList.add('connected');
        } else {
            statusDot.classList.remove('connected');
        }
    }
    
    if (statusText) {
        statusText.textContent = connected ? 'Connected' : 'Disconnected';
    }
}

function handleWebSocketData(data) {
    if (data.type === 'connected') {
        addConsoleLog('✅ ' + data.message, 'success');
        return;
    }
    
    // Handle real-time data from MQTT
    if (data.device_id || data.voltage) {
        updateDashboardWithRealtimeData(data);
    }
}

function updateDashboardWithRealtimeData(data) {
    const voltage = parseFloat(data.voltage || 0);
    const current = parseFloat(data.current || 0);
    const power = parseFloat(data.power || 0);
    const energy = parseFloat(data.energy || 0);
    const frequency = parseFloat(data.frequency || 50);
    const pf = parseFloat(data.power_factor || 1);
    
    // Update stat cards
    updateStatCard('voltageValue', voltage, 2);
    updateStatCard('currentValue', current, 2);
    updateStatCard('powerValue', power, 2);
    updateStatCard('energyValue', energy, 3);
    updateStatCard('frequencyValue', frequency, 1);
    updateStatCard('pfValue', pf, 2);
    
    const now = new Date();
    const timeStr = now.toLocaleTimeString('id-ID');
    updateStatTime('voltageTime', timeStr);
    updateStatTime('currentTime', timeStr);
    updateStatTime('powerTime', timeStr);
    updateStatTime('energyTime', timeStr);
    updateStatTime('frequencyTime', timeStr);
    updateStatTime('pfTime', timeStr);
    
    // Add to history jika dalam range filter
    const timestamp = data.timestamp || now.getTime();
    if (timestamp >= currentFilter.startDate.getTime() && 
        timestamp <= currentFilter.endDate.getTime()) {
        
        const historyItem = {
            timestamp,
            voltage,
            current,
            power,
            energy,
            frequency,
            pf
        };
        
        dataHistory.unshift(historyItem);
        totalRecords++;
        
        updateDataTable();
        updatePaginationControls();
    }
    
    addConsoleLog(
        `📊 V:${voltage.toFixed(2)}V I:${current.toFixed(2)}A P:${power.toFixed(2)}W`,
        'data'
    );
}

function updateDashboardWithDataDirect(historyItem) {
    const voltage = historyItem.voltage || 0;
    const current = historyItem.current || 0;
    const power = historyItem.power || 0;
    const energy = historyItem.energy || 0;
    const frequency = historyItem.frequency || 50;
    const pf = historyItem.pf || 1;
    
    updateStatCard('voltageValue', voltage, 2);
    updateStatCard('currentValue', current, 2);
    updateStatCard('powerValue', power, 2);
    updateStatCard('energyValue', energy, 3);
    updateStatCard('frequencyValue', frequency, 1);
    updateStatCard('pfValue', pf, 2);
    
    const timeStr = new Date(historyItem.timestamp).toLocaleTimeString('id-ID');
    updateStatTime('voltageTime', timeStr);
    updateStatTime('currentTime', timeStr);
    updateStatTime('powerTime', timeStr);
    updateStatTime('energyTime', timeStr);
    updateStatTime('frequencyTime', timeStr);
    updateStatTime('pfTime', timeStr);
}

function updateStatCard(elementId, value, decimals) {
    const el = document.getElementById(elementId);
    if (el) {
        el.textContent = value.toFixed(decimals);
        el.style.transform = 'scale(1.1)';
        setTimeout(() => {
            el.style.transform = 'scale(1)';
        }, 200);
    }
}

function updateStatTime(elementId, timeStr) {
    const el = document.getElementById(elementId);
    if (el) {
        el.textContent = timeStr;
    }
}

// ===== PAGINATION & TABLE =====
function updateDataTable() {
    const tbody = document.getElementById('dataTableBody');
    
    if (dataHistory.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="no-data">No data available</td></tr>';
        document.getElementById('historyCount').textContent = '0';
        document.getElementById('totalData').textContent = '0';
        document.getElementById('totalDataTable').textContent = '0';
        return;
    }
    
    const startIndex = (currentPage - 1) * itemsPerPage;
    const endIndex = Math.min(startIndex + itemsPerPage, dataHistory.length);
    const pageData = dataHistory.slice(startIndex, endIndex);
    
    tbody.innerHTML = pageData.map(item => {
        const date = new Date(item.timestamp);
        return `
        <tr>
            <td>${date.toLocaleString('id-ID')}</td>
            <td>${item.voltage.toFixed(2)}</td>
            <td>${item.current.toFixed(2)}</td>
            <td>${item.power.toFixed(2)}</td>
            <td>${item.energy.toFixed(3)}</td>
            <td>${item.frequency.toFixed(1)}</td>
            <td>${item.pf.toFixed(2)}</td>
        </tr>
    `}).join('');
    
    document.getElementById('historyCount').textContent = `${startIndex + 1}-${endIndex}`;
    document.getElementById('totalData').textContent = dataHistory.length;
    document.getElementById('totalDataTable').textContent = dataHistory.length;
}

function updatePaginationControls() {
    const paginationDiv = document.getElementById('paginationControls');
    if (!paginationDiv) return;
    
    if (dataHistory.length === 0) {
        paginationDiv.innerHTML = '';
        return;
    }
    
    const totalPages = Math.ceil(dataHistory.length / itemsPerPage);
    
    paginationDiv.innerHTML = `
        <div class="pagination">
            <button onclick="goToPage(1)" ${currentPage === 1 ? 'disabled' : ''}>⏮️ First</button>
            <button onclick="goToPage(${currentPage - 1})" ${currentPage === 1 ? 'disabled' : ''}>◀️ Prev</button>
            <span class="page-info">Page ${currentPage} of ${totalPages}</span>
            <button onclick="goToPage(${currentPage + 1})" ${currentPage === totalPages ? 'disabled' : ''}>Next ▶️</button>
            <button onclick="goToPage(${totalPages})" ${currentPage === totalPages ? 'disabled' : ''}>Last ⏭️</button>
        </div>
        <div class="items-per-page">
            <label>Items per page:</label>
            <select onchange="changeItemsPerPage(this.value)">
                <option value="25" ${itemsPerPage === 25 ? 'selected' : ''}>25</option>
                <option value="50" ${itemsPerPage === 50 ? 'selected' : ''}>50</option>
                <option value="100" ${itemsPerPage === 100 ? 'selected' : ''}>100</option>
                <option value="500" ${itemsPerPage === 500 ? 'selected' : ''}>500</option>
            </select>
        </div>
    `;
}

function goToPage(page) {
    const totalPages = Math.ceil(dataHistory.length / itemsPerPage);
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    updateDataTable();
    updatePaginationControls();
}

function changeItemsPerPage(value) {
    itemsPerPage = parseInt(value);
    currentPage = 1;
    updateDataTable();
    updatePaginationControls();
}

// ===== CONSOLE =====
function addConsoleLog(message, type = 'info') {
    const console = document.getElementById('console');
    if (!console) return;
    
    const line = document.createElement('div');
    line.className = `console-line ${type}`;
    
    const timestamp = new Date().toLocaleTimeString('id-ID');
    line.textContent = `[${timestamp}] ${message}`;
    
    console.appendChild(line);
    
    if (autoScroll) {
        console.scrollTop = console.scrollHeight;
    }
    
    const lines = console.querySelectorAll('.console-line');
    if (lines.length > 100) {
        lines[0].remove();
    }
}

function clearConsole() {
    const console = document.getElementById('console');
    console.innerHTML = '';
    addConsoleLog('🗑️ Console cleared', 'info');
}

function toggleAutoScroll() {
    autoScroll = !autoScroll;
    const btn = document.getElementById('btnAutoScroll');
    btn.textContent = `📌 Auto-scroll: ${autoScroll ? 'ON' : 'OFF'}`;
}

function exportConsoleData() {
    if (dataHistory.length === 0) {
        addConsoleLog('⚠️ No data to export', 'warning');
        return;
    }
    
    const dataStr = JSON.stringify(dataHistory, null, 2);
    const dataBlob = new Blob([dataStr], { type: 'application/json' });
    const url = URL.createObjectURL(dataBlob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `wattwise-data-${new Date().toISOString().replace(/:/g, '-')}.json`;
    link.click();
    URL.revokeObjectURL(url);
    
    addConsoleLog('💾 Data exported', 'success');
}

// ===== UTILITIES =====
function updateLastUpdateTime() {
    const now = new Date();
    const el = document.getElementById('lastUpdate');
    if (el) {
        el.textContent = now.toLocaleTimeString('id-ID');
    }
}

function logout() {
    if (ws) {
        ws.close();
    }
    
    sessionStorage.removeItem('wattwise_token');
    sessionStorage.removeItem('wattwise_user');
    
    window.location.href = '/view/login.html';
}

window.addEventListener('beforeunload', () => {
    if (ws) {
        ws.close();
    }
});