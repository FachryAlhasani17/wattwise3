// Authentication utility functions

/**
 * Check if user is authenticated
 * @returns {Object|null} User object if authenticated, null otherwise
 */
function checkAuth() {
    const token = sessionStorage.getItem('wattwise_token');
    const userStr = sessionStorage.getItem('wattwise_user');
    
    if (!token || !userStr) {
        return null;
    }
    
    try {
        const user = JSON.parse(userStr);
        return user;
    } catch (e) {
        console.error('Failed to parse user data:', e);
        return null;
    }
}

/**
 * Get authentication token
 * @returns {string|null} JWT token or null
 */
function getToken() {
    return sessionStorage.getItem('wattwise_token');
}

/**
 * Logout user and redirect to login page
 */
function logout() {
    // Clear session storage
    sessionStorage.removeItem('wattwise_token');
    sessionStorage.removeItem('wattwise_user');
    
    // Redirect to login page
    window.location.href = '/view/login.html';
}

/**
 * Make authenticated API request
 * @param {string} url - API endpoint
 * @param {Object} options - Fetch options
 * @returns {Promise<Response>}
 */
async function authenticatedFetch(url, options = {}) {
    const token = getToken();
    
    if (!token) {
        throw new Error('No authentication token found');
    }
    
    // Add Authorization header
    const headers = {
        ...options.headers,
        'Authorization': `Bearer ${token}`,
    };
    
    const response = await fetch(url, {
        ...options,
        headers,
    });
    
    // If unauthorized, logout
    if (response.status === 401) {
        logout();
        throw new Error('Session expired. Please login again.');
    }
    
    return response;
}

// Export functions for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        checkAuth,
        getToken,
        logout,
        authenticatedFetch,
    };
}