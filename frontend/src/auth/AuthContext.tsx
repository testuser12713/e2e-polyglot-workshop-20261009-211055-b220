import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import {
  ApiError,
  apiFetch,
  EMPLOYEE_STORAGE_KEY,
  getStoredToken,
  TOKEN_STORAGE_KEY,
} from '../api/client'
import type { Employee, LoginResponse } from '../api/types'

interface AuthContextValue {
  token: string | null
  employee: Employee | null
  isAuthenticated: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

function readStoredEmployee(): Employee | null {
  try {
    const raw = window.localStorage.getItem(EMPLOYEE_STORAGE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as Partial<Employee>
    if (typeof parsed.email === 'string' && typeof parsed.name === 'string') {
      return { email: parsed.email, name: parsed.name }
    }
    return null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getStoredToken())
  const [employee, setEmployee] = useState<Employee | null>(() => readStoredEmployee())

  const login = useCallback(async (email: string, password: string) => {
    const result = await apiFetch<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
    try {
      window.localStorage.setItem(TOKEN_STORAGE_KEY, result.token)
      window.localStorage.setItem(EMPLOYEE_STORAGE_KEY, JSON.stringify(result.employee))
    } catch {
      /* storage may be unavailable (private mode); the in-memory session still works */
    }
    setToken(result.token)
    setEmployee(result.employee)
  }, [])

  const logout = useCallback(() => {
    try {
      window.localStorage.removeItem(TOKEN_STORAGE_KEY)
      window.localStorage.removeItem(EMPLOYEE_STORAGE_KEY)
    } catch {
      /* ignore */
    }
    setToken(null)
    setEmployee(null)
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      token,
      employee,
      isAuthenticated: token !== null,
      login,
      logout,
    }),
    [token, employee, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export { ApiError }
