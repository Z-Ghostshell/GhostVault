/**
 * Shared “Ghost in the Shell”–style HUD tokens (same as GIS dashboard gitsUi).
 */
export const gits = {
  card: 'rounded-xl border border-cyan-500/20 bg-slate-950/85 shadow-[0_0_28px_-10px_rgba(34,211,238,0.14),inset_0_1px_0_0_rgba(34,211,238,0.07)]',
  input:
    'rounded-lg border border-cyan-500/25 bg-slate-950/70 px-3 py-1.5 text-sm text-slate-100 shadow-inner placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-1 focus:ring-cyan-500/35',
  inputSm:
    'rounded-lg border border-cyan-500/25 bg-slate-950/70 px-3 py-1.5 text-sm text-slate-100 shadow-inner focus:border-cyan-400 focus:outline-none focus:ring-1 focus:ring-cyan-500/35',
  pageTitle: 'text-2xl font-bold tracking-tight text-slate-100',
  pageSub: 'text-slate-400',
  tableHead:
    'bg-slate-900/90 text-left text-xs font-semibold uppercase tracking-wider text-cyan-200/70',
  tableRow: 'divide-slate-800/90 transition-colors hover:bg-cyan-950/25',
  btnGhost:
    'rounded-lg border border-cyan-500/30 bg-slate-950/60 px-4 py-2 text-sm font-medium text-slate-200 shadow-sm hover:border-cyan-400/50 hover:bg-cyan-950/40',
  btnGhostSm:
    'rounded-lg border border-cyan-500/25 bg-slate-950/50 px-3 py-2 text-sm text-slate-300 hover:border-cyan-400/40 hover:bg-cyan-950/30',
  hudLabel: 'text-[10px] font-semibold uppercase tracking-[0.2em] text-cyan-500/80',
  configErrBanner:
    'rounded-xl border border-amber-500/35 bg-gradient-to-r from-amber-950/50 to-amber-950/30 px-4 py-3 text-sm text-amber-50 shadow-[0_0_24px_-8px_rgba(245,158,11,0.35)]',
} as const
