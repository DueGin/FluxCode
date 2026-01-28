/**
 * User API endpoints
 * Handles user profile management and password changes
 */

import { apiClient } from './client'
import type { User, ChangePasswordRequest } from '@/types'

export interface UiPreferences {
  dashboard_qq_group_popup_disabled: boolean
}

/**
 * Get current user profile
 * @returns User profile data
 */
export async function getProfile(): Promise<User> {
  const { data } = await apiClient.get<User>('/user/profile')
  return data
}

export async function getUiPreferences(): Promise<UiPreferences> {
  const { data } = await apiClient.get<UiPreferences>('/user/ui-preferences')
  return data
}

export async function updateUiPreferences(prefs: Partial<UiPreferences>): Promise<UiPreferences> {
  const { data } = await apiClient.put<UiPreferences>('/user/ui-preferences', prefs)
  return data
}

/**
 * Update current user profile
 * @param profile - Profile data to update
 * @returns Updated user profile data
 */
export async function updateProfile(profile: {
  username?: string
}): Promise<User> {
  const { data } = await apiClient.put<User>('/user', profile)
  return data
}

/**
 * Change current user password
 * @param passwords - Old and new password
 * @returns Success message
 */
export async function changePassword(
  oldPassword: string,
  newPassword: string
): Promise<{ message: string }> {
  const payload: ChangePasswordRequest = {
    old_password: oldPassword,
    new_password: newPassword
  }

  const { data } = await apiClient.put<{ message: string }>('/user/password', payload)
  return data
}

export const userAPI = {
  getProfile,
  getUiPreferences,
  updateUiPreferences,
  updateProfile,
  changePassword
}

export default userAPI
