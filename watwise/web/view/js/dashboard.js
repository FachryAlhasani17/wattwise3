// ===== GLOBAL STATE =====
let ws = null;
let autoScroll = true;
let dataHistory = [];
const MAX_HISTORY = 100000; // Increased to 100k
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;

// Pagination state
let currentPage = 1;
let itemsPerPage = 50;
let totalPages = 1;
let isLoadingMore = false;

// ===== INITIALIZATION =====
window.addEventListener('DOMContentLoaded', () => {
    // Check authentication
    const user = checkAuth();
    if (!user) {
        window.location.href = '/view/login.html';
        return;
    }
    
    // Set username
    document.getElementById('username').textContent = user.username || 'Admin';
    
    // Initialize Dashboard
    addConsoleLog('🚀 Dashboard initialized', 'success');
    
    // ✅ FETCH ALL DATA FROM IOTDB
    fetchAllHistoricalData();
    
    // Initialize WebSocket for real-time updates
    initWebSocket();
    
    // Update time every second
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
        
        // ✅ STRATEGY 1: Try to get data with very high limit
        addConsoleLog('🔍 Strategy 1: Requesting with limit=100000', 'info');
        
        let response = await fetch('/api/energy/data?limit=100000', {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            }
        });
        
        addConsoleLog(`📡 Response status: ${response.status} ${response.statusText}`, 'info');
        
        if (!response.ok) {
            addConsoleLog(`❌ Strategy 1 failed, trying Strategy 2...`, 'warning');
            
            // ✅ STRATEGY 2: Use time range to get ALL data
            // Get data from 1 year ago to now
            const endTime = Date.now();
            const startTime = endTime - (365 * 24 * 60 * 60 * 1000); // 1 year ago
            
            addConsoleLog(`🔍 Strategy 2: Using time range (${new Date(startTime).toISOString()} to ${new Date(endTime).toISOString()})`, 'info');
            
            response = await fetch(`/api/energy/history?device_id=ESP32_PZEM&start_time=${startTime}&end_time=${endTime}&limit=100000`, {
                method: 'GET',
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                }
            });
            
            if (!response.ok) {
                const errorText = await response.text();
                addConsoleLog(`❌ All strategies failed: ${errorText}`, 'error');
                return;
            }
        }
        
        const result = await response.json();
        console.log('🔍 API Full Response:', result);
        
        // Handle different response formats
        let dataArray = [];
        
        if (result.success && result.data) {
            dataArray = result.data;
        } else if (result.data) {
            dataArray = result.data;
        } else if (Array.isArray(result)) {
            dataArray = result;
        }
        
        if (Array.isArray(dataArray) && dataArray.length > 0) {
            addConsoleLog(`✅ Loaded ${dataArray.length} historical records from IoTDB`, 'success');
            
            // Clear existing data
            dataHistory = [];
            
            // Process each data point
            dataArray.forEach(item => {
                addHistoricalDataToTable(item);
            });
            
            addConsoleLog(`✅ Successfully processed ${dataHistory.length} valid records`, 'success');
            
            // Update the latest data to stat cards
            if (dataHistory.length > 0) {
                const latestData = dataHistory[0]; // First item should be latest after processing
                updateDashboardWithDataDirect(latestData, false);
            }
            
            // Calculate pagination
            totalPages = Math.ceil(dataHistory.length / itemsPerPage);
            
            // Update UI
            updateDataTable();
            updatePaginationControls();
            
            addConsoleLog(`📊 Total pages: ${totalPages} | Items per page: ${itemsPerPage}`, 'info');
        } else {
            addConsoleLog('⚠️ No historical data available or empty array', 'warning');
            console.log('Response structure:', result);
        }
        
    } catch (error) {
        console.error('❌ Failed to fetch initial data:', error);
        addConsoleLog(`❌ Failed to load historical data: ${error.message}`, 'error');
    }
}

// ===== ADD HISTORICAL DATA TO ARRAY =====
function addHistoricalDataToTable(data) {
    let timestamp;
    if (typeof data.timestamp === 'number') {
        timestamp = new Date(data.timestamp);
    } else if (typeof data.timestamp === 'string') {
        timestamp = new Date(data.timestamp);
    } else if (data.Timestamp) {
        // Handle capitalized field name
        if (typeof data.Timestamp === 'number') {
            timestamp = new Date(data.Timestamp);
        } else {
            timestamp = new Date(data.Timestamp);
        }
    } else {
        timestamp = new Date();
    }
    
    const voltage = data.voltage || data.Voltage || 0;
    const current = data.current || data.Current || 0;
    const power = data.power || data.Power || 0;
    const energy = data.energy || data.Energy || 0;
    const frequency = data.frequency || data.Frequency || 50;
    const pf = data.power_factor || data.PowerFactor || data.pf || 1;
    
    // Skip invalid data (but be more lenient)
    if (voltage === 0 && current === 0 && power === 0 && energy === 0) {
        return;
    }
    
    const historyItem = {
        timestamp,
        voltage,
        current,
        power,
        energy,
        frequency,
        pf
    };
    
    dataHistory.push(historyItem);
}

// ===== WEBSOCKET FUNCTIONS =====
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    addConsoleLog('🔌 Connecting to WebSocket: ' + wsUrl, 'info');
    
    try {
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function(event) {
            addConsoleLog('✅ WebSocket connected successfully', 'success');
            updateConnectionStatus(true);
            reconnectAttempts = 0;
        };
        
        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                addConsoleLog('📨 Real-time data received from WebSocket', 'success');
                handleWebSocketData(data);
            } catch (error) {
                console.error('❌ Parse error:', error);
                addConsoleLog('❌ Error parsing WebSocket data: ' + error.message, 'error');
            }
        };
        
        ws.onerror = function(error) {
            addConsoleLog('❌ WebSocket error occurred', 'error');
            updateConnectionStatus(false);
        };
        
        ws.onclose = function(event) {
            addConsoleLog('🔌 WebSocket disconnected (Code: ' + event.code + ')', 'warning');
            updateConnectionStatus(false);
            
            if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000);
                addConsoleLog(`🔄 Reconnecting in ${delay/1000}s... (Attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`, 'info');
                setTimeout(initWebSocket, delay);
            } else {
                addConsoleLog('❌ Max reconnection attempts reached. Please refresh the page.', 'error');
            }
        };
        
    } catch (error) {
        addConsoleLog('❌ Failed to create WebSocket: ' + error.message, 'error');
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
        addConsoleLog('📊 Received real-time data from MQTT', 'data');
        updateDashboardWithData(data);
    }
    
    updateLastUpdateTime();
}

function updateDashboardWithData(data, addToHistory = true) {
    const voltage = data.voltage || data.Voltage || 0;
    const current = data.current || data.Current || 0;
    const power = data.power || data.Power || 0;
    const energy = data.energy || data.Energy || 0;
    const frequency = data.frequency || data.Frequency || 50;
    const pf = data.power_factor || data.PowerFactor || data.pf || 1;
    
    if (voltage === 0 && current === 0 && power === 0) {
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
        
        // Recalculate pagination
        totalPages = Math.ceil(dataHistory.length / itemsPerPage);
        
        updateDataTable();
        updatePaginationControls();
        
        addConsoleLog(
            `📊 V: ${voltage.toFixed(2)}V | I: ${current.toFixed(2)}A | P: ${power.toFixed(2)}W | E: ${energy.toFixed(3)}kWh`,
            'data'
        );
    }
}

// Direct update without adding to history (for initial load)
function updateDashboardWithDataDirect(historyItem, addToHistory = false) {
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

// ===== PAGINATION & TABLE UPDATE =====
function updateDataTable() {
    const tbody = document.getElementById('dataTableBody');
    
    if (dataHistory.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="no-data">No data available</td></tr>';
        document.getElementById('historyCount').textContent = '0';
        document.getElementById('totalData').textContent = '0';
        return;
    }
    
    // Calculate start and end index for current page
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
    
    let html = `
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
    
    paginationDiv.innerHTML = html;
}

function goToPage(page) {
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    updateDataTable();
    updatePaginationControls();
    addConsoleLog(`📄 Navigated to page ${currentPage}`, 'info');
}

function changeItemsPerPage(value) {
    itemsPerPage = parseInt(value);
    currentPage = 1;
    totalPages = Math.ceil(dataHistory.length / itemsPerPage);
    updateDataTable();
    updatePaginationControls();
    addConsoleLog(`📊 Changed items per page to ${itemsPerPage}`, 'info');
}

// ===== CONSOLE FUNCTIONS =====
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
    
    addConsoleLog('💾 Data exported successfully', 'success');
}

// ===== UTILITY FUNCTIONS =====
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