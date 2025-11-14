#!/bin/bash

echo "==================================="
echo "🔧 Memperbaiki Mosquitto Port 1883"
echo "==================================="
echo ""

# 1. Cek proses yang menggunakan port 1883
echo "📋 Step 1: Cek proses di port 1883..."
sudo lsof -i :1883

echo ""
echo "⏸️ Step 2: Menghentikan semua proses Mosquitto..."
# Stop semua proses mosquitto
sudo systemctl stop mosquitto 2>/dev/null
sudo pkill -9 mosquitto 2>/dev/null
sleep 2

# Cek apakah masih ada yang menggunakan port
echo ""
echo "🔍 Step 3: Verifikasi port 1883..."
PORT_CHECK=$(sudo lsof -i :1883 | grep LISTEN)

if [ -z "$PORT_CHECK" ]; then
    echo "✅ Port 1883 sudah bebas!"
else
    echo "⚠️ Masih ada proses menggunakan port 1883:"
    echo "$PORT_CHECK"
    echo ""
    echo "💀 Paksa kill proses tersebut..."
    
    # Ambil PID dan kill paksa
    PID=$(sudo lsof -t -i :1883)
    if [ ! -z "$PID" ]; then
        sudo kill -9 $PID
        echo "✅ Proses $PID dihentikan paksa"
    fi
fi

echo ""
echo "🚀 Step 4: Jalankan Mosquitto dengan verbose mode..."
echo "   (Tekan Ctrl+C untuk menghentikan)"
echo ""

# Jalankan mosquitto dengan verbose
sudo mosquitto -c /etc/mosquitto/mosquitto.conf -v

# Jika script mencapai sini (user menekan Ctrl+C)
echo ""
echo "🛑 Mosquitto dihentikan"