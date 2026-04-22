import { ActivitySquare, Bug, Cog, Home, KeyRound, Settings2, Terminal, Unlock } from 'lucide-react'
import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import SessionGateModal from '../components/SessionGateModal'
import { SessionUIProvider } from '../context/SessionUIContext'
import { getStoredBearer, setStoredBearer } from '../api/client'
import { getApiDisplayLabel, isRetrieveDebugNavVisible } from '../config'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-all ${
    isActive
      ? 'border border-cyan-400/35 bg-cyan-950/50 text-cyan-200 shadow-[0_0_20px_-6px_rgba(34,211,238,0.45)]'
      : 'border border-transparent text-slate-400 hover:border-slate-600 hover:bg-slate-900/60 hover:text-slate-200'
  }`

function maskToken(raw: string): string {
  const t = raw.trim()
  if (t.length <= 12) return '••••••••'
  return `${t.slice(0, 6)}…${t.slice(-4)}`
}

export default function AppShell() {
  const [stored, setStored] = useState<string | null>(null)
  const [sessionModalExtraOpen, setSessionModalExtraOpen] = useState(false)
  const [showRetrieveDebug, setShowRetrieveDebug] = useState(() => isRetrieveDebugNavVisible())

  useEffect(() => {
    const sync = () => setStored(getStoredBearer())
    sync()
    window.addEventListener('ghostvault-bearer-changed', sync)
    return () => window.removeEventListener('ghostvault-bearer-changed', sync)
  }, [])

  useEffect(() => {
    const sync = () => setShowRetrieveDebug(isRetrieveDebugNavVisible())
    window.addEventListener('ghostvault-retrieve-debug-changed', sync)
    return () => window.removeEventListener('ghostvault-retrieve-debug-changed', sync)
  }, [])

  const hasSession = Boolean(stored?.trim())
  const sessionModalOpen = !hasSession || sessionModalExtraOpen

  const openSessionModal = () => setSessionModalExtraOpen(true)

  const clearToken = () => {
    setStoredBearer(null)
    setStored(null)
    setSessionModalExtraOpen(false)
  }

  return (
    <SessionUIProvider value={{ openSessionModal }}>
      <div className="gits-app-bg flex h-screen min-h-0 w-full overflow-hidden">
        <aside className="hidden h-full min-h-0 w-64 shrink-0 flex-col overflow-y-auto overflow-x-hidden border-r border-cyan-500/15 bg-slate-950/70 backdrop-blur-sm md:flex">
          <div className="border-b border-cyan-500/10 p-4">
            <div className="font-mono text-[10px] font-semibold uppercase tracking-[0.35em] text-cyan-500/90">
              Ghost Vault
            </div>
            <div className="mt-1 text-lg font-semibold tracking-wide text-slate-100">Dashboard</div>
            <div className="mt-2 font-mono text-[10px] uppercase tracking-widest text-fuchsia-400/70">
              Memory · gvsvd
            </div>
          </div>
          <nav className="flex flex-col gap-0.5 p-3">
            <NavLink to="/" className={navLinkClass} end>
              <Home className="h-4 w-4 shrink-0 text-cyan-400/90" />
              Overview
            </NavLink>
            <NavLink to="/config" className={navLinkClass}>
              <Cog className="h-4 w-4 shrink-0 text-cyan-400/90" />
              Config
            </NavLink>
            <NavLink to="/activity" className={navLinkClass}>
              <ActivitySquare className="h-4 w-4 shrink-0 text-cyan-400/90" />
              Activity
            </NavLink>
            <NavLink to="/api-console" className={navLinkClass}>
              <Terminal className="h-4 w-4 shrink-0 text-fuchsia-400/80" />
              API console
            </NavLink>
            {showRetrieveDebug ? (
              <NavLink to="/retrieve-debug" className={navLinkClass}>
                <Bug className="h-4 w-4 shrink-0 text-fuchsia-400/80" />
                Retrieve debug
              </NavLink>
            ) : null}
            <button
              type="button"
              className={navLinkClass({ isActive: false })}
              onClick={openSessionModal}
            >
              <Unlock className="h-4 w-4 shrink-0 text-cyan-400/90" />
              Session / unlock
            </button>
          </nav>
          <div className="mt-auto space-y-2 border-t border-cyan-500/10 p-3">
            <div className="font-mono text-[10px] font-medium uppercase tracking-wider text-slate-500">API base</div>
            <div className="break-all font-mono text-[11px] text-cyan-600/90">{getApiDisplayLabel()}</div>
            <p className="text-[11px] leading-snug text-slate-500">
              Use the session dialog to paste a token or unlock. <code className="text-slate-400">gvctl unlock</code> still
              works from the CLI.
            </p>
          </div>
        </aside>

        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <header className="flex shrink-0 flex-wrap items-center gap-3 border-b border-cyan-500/15 bg-slate-950/80 px-3 py-2 backdrop-blur-sm md:px-4 md:py-3">
            <KeyRound className="hidden h-4 w-4 text-cyan-500/80 md:block" />
            {hasSession ? (
              <>
                <span className="text-xs text-slate-500">Session</span>
                <span className="font-mono text-[11px] text-slate-300">{maskToken(stored!)}</span>
                <button
                  type="button"
                  onClick={openSessionModal}
                  className="inline-flex items-center gap-1 rounded-lg border border-cyan-500/30 bg-cyan-950/35 px-2.5 py-1 text-xs font-medium text-cyan-100 hover:bg-cyan-950/55"
                >
                  <Settings2 className="h-3.5 w-3.5" />
                  Change
                </button>
                <button
                  type="button"
                  onClick={clearToken}
                  className="text-xs text-amber-400/90 hover:text-amber-300"
                >
                  Sign out
                </button>
              </>
            ) : (
              <span className="text-xs text-amber-500/90">Sign in via the dialog —</span>
            )}
          </header>

          <main
            className="min-h-0 flex-1 overflow-y-auto p-4 md:p-8"
            inert={!hasSession ? true : undefined}
            aria-hidden={!hasSession ? true : undefined}
          >
            <div className="animate-fade-in mx-auto max-w-6xl">
              <Outlet />
            </div>
          </main>
        </div>

        <SessionGateModal
          open={sessionModalOpen}
          forced={!hasSession}
          onClose={() => setSessionModalExtraOpen(false)}
        />
      </div>
    </SessionUIProvider>
  )
}
