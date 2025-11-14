document.getElementById('loginForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    const submitBtn = e.target.querySelector('button[type="submit"]');
    
    // Disable button
    submitBtn.disabled = true;
    submitBtn.textContent = 'Memproses...';
    
    try {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ username, password })
        });
        
        const result = await response.json();
        
        if (result.success && result.data && result.data.token) {
            // Save auth data
            localStorage.setItem('token', result.data.token);
            localStorage.setItem('username', username);
            
            console.log('✅ Login successful');
            
            // Redirect to dashboard
            window.location.href = '/view/dashboard.html';
        } else {
            alert(result.message || 'Login gagal');
            submitBtn.disabled = false;
            submitBtn.textContent = 'Masuk';
        }
    } catch (error) {
        console.error('❌ Login error:', error);
        alert('Terjadi kesalahan saat login');
        submitBtn.disabled = false;
        submitBtn.textContent = 'Masuk';
    }
});