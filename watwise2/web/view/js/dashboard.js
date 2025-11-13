// ===== GLOBAL STATE =====
let ws = null;
let autoScroll = true;
let dataHistory = [];
const MAX_HISTORY = 50;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 5;

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
    
    // ✅ FETCH INITIAL DATA FROM IOTDB
    fetchInitialData();
    
    // Initialize WebSocket for real-time updates
    initWebSocket();
    
    // Update time every second
    setInterval(updateLastUpdateTime, 1000);
});

// ===== FETCH INITIAL DATA FROM IOTDB =====
async function fetchInitialData() {
    addConsoleLog('📥 Fetching initial data from IoTDB...', 'info');
    
    try {
        const token = getToken();
        const response = await fetch('/api/energy/data?limit=50', {
            method: 'GET',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            }
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const result = await response.json();
        
        if (result.success && result.data && Array.isArray(result.data)) {
            addConsoleLog(`✅ Loaded ${result.data.length} historical records from IoTDB`, 'success');
            
            // Process each data point
            result.data.forEach(item => {
                addHistoricalDataToTable(item);
            });
            
            // Update the latest data to stat cards
            if (result.data.length > 0) {
                const latestData = result.data[0]; // First item is the latest
                updateDashboardWithData(latestData, false); // false = don't add to history (already added)
            }
            
            updateDataTable();
            
        } else {
            addConsoleLog('⚠️ No historical data available', 'warning');
        }
        
    } catch (error) {
        console.error('❌ Failed to fetch initial data:', error);
        addConsoleLog(`❌ Failed to load historical data: ${error.message}`, 'error');
    }
}

// ===== ADD HISTORICAL DATA TO TABLE (WITHOUT LOGGING TO CONSOLE) =====
function addHistoricalDataToTable(data) {
    // Convert timestamp to Date object
    let timestamp;
    if (typeof data.timestamp === 'number') {
        timestamp = new Date(data.timestamp);
    } else if (typeof data.timestamp === 'string') {
        timestamp = new Date(data.timestamp);
    } else {
        timestamp = new Date();
    }
    
    const voltage = data.voltage || 0;
    const current = data.current || 0;
    const power = data.power || 0;
    const energy = data.energy || 0;
    const frequency = data.frequency || 50;
    const pf = data.power_factor || data.pf || 1;
    
    // Skip invalid data
    if (voltage === 0 && current === 0 && power === 0) {
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
            
            // Auto reconnect
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
    // Handle connection message
    if (data.type === 'connected') {
        addConsoleLog('✅ ' + data.message, 'success');
        return;
    }
    
    // Handle different data formats from backend
    if (Array.isArray(data)) {
        data.forEach(item => updateDashboardWithData(item));
    } else if (data.data && Array.isArray(data.data)) {
        data.data.forEach(item => updateDashboardWithData(item));
    } else if (data.device_id || data.DeviceID || data.voltage || data.Voltage) {
        // Single data object (realtime from MQTT)
        addConsoleLog('📊 Received real-time data from MQTT', 'data');
        updateDashboardWithData(data);
    }
    
    updateLastUpdateTime();
}

function updateDashboardWithData(data, addToHistory = true) {
    // Extract values with fallback for different field names
    const voltage = data.voltage || data.Voltage || 0;
    const current = data.current || data.Current || 0;
    const power = data.power || data.Power || 0;
    const energy = data.energy || data.Energy || 0;
    const frequency = data.frequency || data.Frequency || 50;
    const pf = data.power_factor || data.PowerFactor || data.pf || 1;
    
    // Skip if all values are zero (invalid data)
    if (voltage === 0 && current === 0 && power === 0) {
        return;
    }
    
    // Update stat cards
    updateStatCard('voltageValue', voltage, 2);
    updateStatCard('currentValue', current, 2);
    updateStatCard('powerValue', power, 2);
    updateStatCard('energyValue', energy, 3);
    updateStatCard('frequencyValue', frequency, 1);
    updateStatCard('pfValue', pf, 2);
    
    // Update timestamps
    const now = new Date();
    const timeStr = now.toLocaleTimeString('id-ID');
    updateStatTime('voltageTime', timeStr);
    updateStatTime('currentTime', timeStr);
    updateStatTime('powerTime', timeStr);
    updateStatTime('energyTime', timeStr);
    updateStatTime('frequencyTime', timeStr);
    updateStatTime('pfTime', timeStr);
    
    // Add to history only if this is new real-time data
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
        
        updateDataTable();
        
        // Log to console for real-time data only
        addConsoleLog(
            `📊 V: ${voltage.toFixed(2)}V | I: ${current.toFixed(2)}A | P: ${power.toFixed(2)}W | E: ${energy.toFixed(3)}kWh`,
            'data'
        );
    }
}

function updateStatCard(elementId, value, decimals) {
    const el = document.getElementById(elementId);
    if (el) {
        el.textContent = value.toFixed(decimals);
        
        // Add animation effect
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

function updateDataTable() {
    const tbody = document.getElementById('dataTableBody');
    const recentData = dataHistory.slice(0, 50); // Show up to 50 records
    
    if (recentData.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="no-data">No data available</td></tr>';
        return;
    }
    
    tbody.innerHTML = recentData.map(item => `
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
    
    document.getElementById('historyCount').textContent = recentData.length;
    document.getElementById('totalData').textContent = dataHistory.length;
}

// ===== CONSOLE FUNCTIONS =====
function addConsoleLog(message, type = 'info') {
    const console = document.getElementById('console');
    const line = document.createElement('div');
    line.className = `console-line ${type}`;
    
    const timestamp = new Date().toLocaleTimeString('id-ID');
    line.textContent = `[${timestamp}] ${message}`;
    
    console.appendChild(line);
    
    // Auto scroll if enabled
    if (autoScroll) {
        console.scrollTop = console.scrollHeight;
    }
    
    // Limit console lines to prevent memory issues
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
    
    // Clear session storage
    sessionStorage.removeItem('wattwise_token');
    sessionStorage.removeItem('wattwise_user');
    
    // Redirect to login
    window.location.href = '/view/login.html';
}

// ===== CLEANUP =====
window.addEventListener('beforeunload', () => {
    if (ws) {
        ws.close();
    }
});