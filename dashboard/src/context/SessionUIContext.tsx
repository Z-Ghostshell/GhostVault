import { createContext, useContext, type ReactNode } from 'react'

export type SessionUIValue = {
  openSessionModal: () => void
}

const SessionUIContext = createContext<SessionUIValue | null>(null)

export function SessionUIProvider({
  children,
  value,
}: {
  children: ReactNode
  value: SessionUIValue
}) {
  return <SessionUIContext.Provider value={value}>{children}</SessionUIContext.Provider>
}

export function useSessionUI(): SessionUIValue {
  const v = useContext(SessionUIContext)
  if (!v) {
    throw new Error('useSessionUI outside SessionUIProvider')
  }
  return v
}
