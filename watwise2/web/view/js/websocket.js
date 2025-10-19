// WebSocket connection handler for real-time energy data

class EnergyWebSocket {
    constructor(url, onDataCallback, onStatusCallback) {
        this.url = url;
        this.onDataCallback = onDataCallback;
        this.onStatusCallback = onStatusCallback;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 3000; // 3 seconds
        this.isIntentionallyClosed = false;
    }

    connect() {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            console.log('⚠️ WebSocket already connected');
            return;
        }

        console.log('🔌 Connecting to WebSocket:', this.url);
        
        try {
            this.ws = new WebSocket(this.url);
            
            this.ws.onopen = () => this.handleOpen();
            this.ws.onmessage = (event) => this.handleMessage(event);
            this.ws.onerror = (error) => this.handleError(error);
            this.ws.onclose = (event) => this.handleClose(event);
        } catch (error) {
            console.error('❌ Failed to create WebSocket:', error);
            this.scheduleReconnect();
        }
    }

    handleOpen() {
        console.log('✅ WebSocket connected');
        this.reconnectAttempts = 0;
        
        if (this.onStatusCallback) {
            this.onStatusCallback(true);
        }
    }

    handleMessage(event) {
        try {
            const data = JSON.parse(event.data);
            console.log('📨 Received data:', data);
            
            if (this.onDataCallback) {
                this.onDataCallback(data);
            }
        } catch (error) {
            console.error('❌ Failed to parse WebSocket message:', error);
        }
    }

    handleError(error) {
        console.error('❌ WebSocket error:', error);
        
        if (this.onStatusCallback) {
            this.onStatusCallback(false);
        }
    }

    handleClose(event) {
        console.log('🔌 WebSocket closed:', event.code, event.reason);
        
        if (this.onStatusCallback) {
            this.onStatusCallback(false);
        }

        // Only reconnect if not intentionally closed
        if (!this.isIntentionallyClosed) {
            this.scheduleReconnect();
        }
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('❌ Max reconnection attempts reached');
            return;
        }

        this.reconnectAttempts++;
        console.log(`🔄 Reconnecting in ${this.reconnectDelay / 1000}s... (Attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

        setTimeout(() => {
            this.connect();
        }, this.reconnectDelay);
    }

    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(data));
        } else {
            console.warn('⚠️ WebSocket not connected, cannot send data');
        }
    }

    close() {
        this.isIntentionallyClosed = true;
        
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        
        console.log('🔌 WebSocket connection closed');
    }
}

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = EnergyWebSocket;
}