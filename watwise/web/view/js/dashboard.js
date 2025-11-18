// ===== GLOBAL STATE =====
let ws = null;
let autoScroll = true;
let dataHistory = [];
const MAX_HISTORY = 100000;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;

// Pagination state
let currentPage = 1;
let itemsPerPage = 50;
let totalPages = 1;

// ===== INITIALIZATION =====
window.addEventListener('DOMContentLoaded', () => {
    const user = checkAuth();
    if (!user) {
        window.location.href = '/view/login.html';
        return;
    }
    
    document.getElementById('username').textContent = user.username || 'Admin';
    
    addConsoleLog('🚀 Dashboard initialized', 'success');
    
    fetchAllHistoricalData();
    initWebSocket();
    
    setInterval(updateLastUpdateTime, 1000);
});

// ===== FETCH ALL HISTORICAL DATA FROM IOTDB =====
async function fetchAllHistoricalData() {
    addConsoleLog('📥 Fetching ALL historical data from IoTDB...', 'info');
    
    try {
        const token = getToken();
        
        if (!token) {
            addConsoleLog('⚠️ No authentication token found', 'warning');
            return;
        }
        
        addConsoleLog('🔍 Requesting ALL data from backend (limit=0)', 'info');
        
        let response = await fetch('/api/energy/data?limit=0', {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            }
        });
        
        addConsoleLog(`📡 Response status: ${response.status} ${response.statusText}`, 'info');
        
        if (!response.ok) {
            const errorText = await response.text();
            addConsoleLog(`❌ Failed to fetch data: ${errorText}`, 'error');
            document.getElementById('dataTableBody').innerHTML = 
                '<tr><td colspan="7" class="no-data">Failed to load data from IoTDB. Please check server connection.</td></tr>';
            return;
        }
        
        const result = await response.json();
        console.log('🔍 API Response:', {
            hasSuccess: 'success' in result,
            successValue: result.success,
            hasData: 'data' in result,
            dataType: Array.isArray(result.data) ? 'array' : typeof result.data,
            dataLength: Array.isArray(result.data) ? result.data.length : 'N/A'
        });
        
        let dataArray = [];
        
        if (result.success === true && result.data) {
            dataArray = result.data;
            addConsoleLog('✅ Parsed response format: success=true', 'info');
        } else if (result.success === false) {
            addConsoleLog(`❌ API error: ${result.message || result.error}`, 'error');
            document.getElementById('dataTableBody').innerHTML = 
                `<tr><td colspan="7" class="no-data">Error: ${result.message || result.error}</td></tr>`;
            return;
        } else if (result.data) {
            dataArray = result.data;
            addConsoleLog('✅ Parsed response format: data field', 'info');
        } else if (Array.isArray(result)) {
            dataArray = result;
            addConsoleLog('✅ Parsed response format: direct array', 'info');
        }
        
        if (!Array.isArray(dataArray)) {
            addConsoleLog(`⚠️ Invalid data format (expected array, got ${typeof dataArray})`, 'warning');
            console.log('Data received:', dataArray);
            document.getElementById('dataTableBody').innerHTML = 
                '<tr><td colspan="7" class="no-data">Invalid data format from server</td></tr>';
            return;
        }
        
        if (dataArray.length === 0) {
            addConsoleLog('⚠️ No data in IoTDB', 'warning');
            document.getElementById('dataTableBody').innerHTML = 
                '<tr><td colspan="7" class="no-data">No data available. Start collecting from ESP32.</td></tr>';
            return;
        }
        
        addConsoleLog(`✅ Loaded ${dataArray.length} records from IoTDB`, 'success');
        
        dataHistory = [];
        
        let validCount = 0;
        let skippedCount = 0;
        
        dataArray.forEach((item, index) => {
            if (addHistoricalDataToTable(item)) {
                validCount++;
            } else {
                skippedCount++;
                if (skippedCount <= 5) {
                    console.warn(`Skipped item ${index}:`, item);
                }
            }
        });
        
        if (skippedCount > 0) {
            addConsoleLog(`⚠️ Skipped ${skippedCount} invalid records`, 'warning');
        }
        
        addConsoleLog(`✅ Processed ${validCount}/${dataArray.length} valid records`, 'success');
        
        if (dataHistory.length === 0) {
            addConsoleLog('⚠️ No valid data after processing', 'warning');
            document.getElementById('dataTableBody').innerHTML = 
                '<tr><td colspan="7" class="no-data">All records invalid</td></tr>';
            return;
        }
        
        const latestData = dataHistory[0];
        updateDashboardWithDataDirect(latestData, false);
        addConsoleLog(`📊 Updated with latest: ${latestData.timestamp.toLocaleString()}`, 'success');
        
        totalPages = Math.ceil(dataHistory.length / itemsPerPage);
        
        updateDataTable();
        updatePaginationControls();
        
        addConsoleLog(`📊 Summary:`, 'success');
        addConsoleLog(`   • Total: ${dataHistory.length} records`, 'info');
        addConsoleLog(`   • Pages: ${totalPages}`, 'info');
        addConsoleLog(`   • Per page: ${itemsPerPage}`, 'info');
        
    } catch (error) {
        console.error('❌ Fetch error:', error);
        addConsoleLog(`❌ Failed: ${error.message}`, 'error');
        document.getElementById('dataTableBody').innerHTML = 
            `<tr><td colspan="7" class="no-data">Error: ${error.message}</td></tr>`;
    }
}

// ===== ADD HISTORICAL DATA WITH VALIDATION =====
function addHistoricalDataToTable(data) {
    let timestamp;
    const ts = data.timestamp || data.Timestamp;
    
    if (typeof ts === 'number') {
        timestamp = new Date(ts);
    } else if (typeof ts === 'string') {
        timestamp = new Date(ts);
    } else {
        console.warn('Missing timestamp:', data);
        return false;
    }
    
    if (isNaN(timestamp.getTime())) {
        console.warn('Invalid date:', ts);
        return false;
    }
    
    const voltage = parseFloat(data.voltage || data.Voltage || 0);
    const current = parseFloat(data.current || data.Current || 0);
    const power = parseFloat(data.power || data.Power || 0);
    const energy = parseFloat(data.energy || data.Energy || 0);
    const frequency = parseFloat(data.frequency || data.Frequency || 50);
    const pf = parseFloat(data.power_factor || data.PowerFactor || data.pf || 1);
    
    if ((isNaN(voltage) || voltage === 0) && 
        (isNaN(current) || current === 0) && 
        (isNaN(power) || power === 0) && 
        (isNaN(energy) || energy === 0)) {
        return false;
    }
    
    const historyItem = {
        timestamp,
        voltage: isNaN(voltage) ? 0 : voltage,
        current: isNaN(current) ? 0 : current,
        power: isNaN(power) ? 0 : power,
        energy: isNaN(energy) ? 0 : energy,
        frequency: isNaN(frequency) ? 50 : frequency,
        pf: isNaN(pf) ? 1 : pf
    };
    
    dataHistory.push(historyItem);
    return true;
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
                addConsoleLog('❌ Parse error: ' + error.message, 'error');
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
                addConsoleLog(`🔄 Reconnecting in ${delay/1000}s... (${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`, 'info');
                setTimeout(initWebSocket, delay);
            } else {
                addConsoleLog('❌ Max reconnect attempts. Refresh page.', 'error');
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

// ===== DATA HANDLING =====
function handleWebSocketData(data) {
    if (data.type === 'connected') {
        addConsoleLog('✅ ' + data.message, 'success');
        return;
    }
    
    if (Array.isArray(data)) {
        data.forEach(item => updateDashboardWithData(item));
    } else if (data.data && Array.isArray(data.data)) {
        data.data.forEach(item => updateDashboardWithData(item));
    } else if (data.device_id || data.DeviceID || data.voltage || data.Voltage) {
        addConsoleLog('📊 Real-time data from MQTT', 'data');
        updateDashboardWithData(data);
    }
    
    updateLastUpdateTime();
}

function updateDashboardWithData(data, addToHistory = true) {
    const voltage = parseFloat(data.voltage || data.Voltage || 0);
    const current = parseFloat(data.current || data.Current || 0);
    const power = parseFloat(data.power || data.Power || 0);
    const energy = parseFloat(data.energy || data.Energy || 0);
    const frequency = parseFloat(data.frequency || data.Frequency || 50);
    const pf = parseFloat(data.power_factor || data.PowerFactor || data.pf || 1);
    
    if (voltage === 0 && current === 0 && power === 0 && energy === 0) {
        return;
    }
    
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
    
    if (addToHistory) {
        const historyItem = {
            timestamp: now,
            voltage,
            current,
            power,
            energy,
            frequency,
            pf
        };
        
        dataHistory.unshift(historyItem);
        if (dataHistory.length > MAX_HISTORY) {
            dataHistory = dataHistory.slice(0, MAX_HISTORY);
        }
        
        totalPages = Math.ceil(dataHistory.length / itemsPerPage);
        
        updateDataTable();
        updatePaginationControls();
        
        addConsoleLog(
            `📊 V:${voltage.toFixed(2)}V I:${current.toFixed(2)}A P:${power.toFixed(2)}W E:${energy.toFixed(3)}kWh`,
            'data'
        );
    }
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
    
    const timeStr = historyItem.timestamp.toLocaleTimeString('id-ID');
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
        return;
    }
    
    const startIndex = (currentPage - 1) * itemsPerPage;
    const endIndex = Math.min(startIndex + itemsPerPage, dataHistory.length);
    const pageData = dataHistory.slice(startIndex, endIndex);
    
    tbody.innerHTML = pageData.map(item => `
        <tr>
            <td>${item.timestamp.toLocaleString('id-ID')}</td>
            <td>${item.voltage.toFixed(2)}</td>
            <td>${item.current.toFixed(2)}</td>
            <td>${item.power.toFixed(2)}</td>
            <td>${item.energy.toFixed(3)}</td>
            <td>${item.frequency.toFixed(1)}</td>
            <td>${item.pf.toFixed(2)}</td>
        </tr>
    `).join('');
    
    document.getElementById('historyCount').textContent = `${startIndex + 1}-${endIndex}`;
    document.getElementById('totalData').textContent = dataHistory.length;
}

function updatePaginationControls() {
    const paginationDiv = document.getElementById('paginationControls');
    if (!paginationDiv) return;
    
    if (dataHistory.length === 0) {
        paginationDiv.innerHTML = '';
        return;
    }
    
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
                <option value="1000" ${itemsPerPage === 1000 ? 'selected' : ''}>1000</option>
            </select>
        </div>
    `;
}

function goToPage(page) {
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    updateDataTable();
    updatePaginationControls();
    addConsoleLog(`📄 Page ${currentPage}`, 'info');
}

function changeItemsPerPage(value) {
    itemsPerPage = parseInt(value);
    currentPage = 1;
    totalPages = Math.ceil(dataHistory.length / itemsPerPage);
    updateDataTable();
    updatePaginationControls();
    addConsoleLog(`📊 Items per page: ${itemsPerPage}`, 'info');
}

// ===== CONSOLE =====
function addConsoleLog(message, type = 'info') {
    const console = document.getElementById('console');
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
    addConsoleLog(`📌 Auto-scroll ${autoScroll ? 'enabled' : 'disabled'}`, 'info');
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

// ===== CLEANUP =====
window.addEventListener('beforeunload', () => {
    if (ws) {
        ws.close();
    }
});