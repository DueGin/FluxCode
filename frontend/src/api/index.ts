/**
 * API Client for FluxCode Backend
 * Central export point for all API modules
 */

// Re-export the HTTP client
export { apiClient } from './client'

// Auth API
export { authAPI } from './auth'

// User APIs
export { keysAPI } from './keys'
export { usageAPI } from './usage'
export { userAPI } from './user'
export { redeemAPI, type RedeemHistoryItem } from './redeem'
export { userGroupsAPI } from './groups'
export { pricingPlansAPI } from './pricingPlans'

// Admin APIs
export { adminAPI } from './admin'

// Default export
export { default } from './client'
