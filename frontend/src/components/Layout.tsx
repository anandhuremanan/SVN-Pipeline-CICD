import React from 'react';
import { 
  GitBranch, 
  Play, 
  ClockCounterClockwise, 
  FirstAid, 
  Cube,
  Spinner,
  Sun,
  Moon
} from '@phosphor-icons/react';

interface LayoutProps {
  currentTab: 'pipelines' | 'build-details' | 'doctor';
  selectedJobId: string | null;
  onSelectTab: (tab: 'pipelines' | 'build-details' | 'doctor', jobId?: string) => void;
  onOpenRunModal: () => void;
  activeCount: number;
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
  children: React.ReactNode;
}

export const Layout: React.FC<LayoutProps> = ({
  currentTab,
  selectedJobId,
  onSelectTab,
  onOpenRunModal,
  activeCount,
  theme,
  onToggleTheme,
  children,
}) => {
  return (
    <div className="flex h-screen app-bg-main app-text-primary font-sans overflow-hidden select-none transition-colors duration-200">
      {/* Left Sidebar */}
      <aside className="w-64 app-bg-card border-r app-border flex flex-col justify-between flex-shrink-0 transition-colors duration-200">
        <div>
          {/* Header Branding */}
          <div className="h-16 px-5 border-b app-border flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-md app-badge flex items-center justify-center font-bold">
                <GitBranch size={20} weight="bold" />
              </div>
              <div>
                <h1 className="font-bold text-sm app-text-primary tracking-wider uppercase leading-tight">GBS Pipelines</h1>
                <p className="text-[11px] app-text-muted font-mono">Monochrome Engine</p>
              </div>
            </div>
          </div>

          {/* Navigation Links */}
          <nav className="p-3 space-y-1">
            <button
              onClick={() => onSelectTab('pipelines')}
              className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-md text-xs font-semibold tracking-wide uppercase transition-all ${
                currentTab === 'pipelines' && !selectedJobId
                  ? 'app-badge font-bold shadow-sm'
                  : 'app-text-muted hover:app-text-primary app-bg-hover'
              }`}
            >
              <div className="flex items-center gap-3">
                <ClockCounterClockwise size={16} weight={currentTab === 'pipelines' ? 'fill' : 'regular'} />
                <span>Pipelines</span>
              </div>
              {activeCount > 0 && (
                <span className="flex items-center gap-1 app-bg-card app-text-primary text-[10px] px-2 py-0.5 rounded border app-border font-mono">
                  <Spinner size={10} className="animate-spin" />
                  {activeCount}
                </span>
              )}
            </button>

            {selectedJobId && (
              <button
                onClick={() => onSelectTab('build-details', selectedJobId)}
                className={`w-full flex items-center gap-3 px-3.5 py-2.5 rounded-md text-xs font-semibold tracking-wide uppercase transition-all ${
                  currentTab === 'build-details'
                    ? 'app-badge font-bold shadow-sm'
                    : 'app-text-muted hover:app-text-primary app-bg-hover'
                }`}
              >
                <Cube size={16} weight={currentTab === 'build-details' ? 'fill' : 'regular'} />
                <span className="truncate">Job: {selectedJobId.substring(0, 14)}...</span>
              </button>
            )}

            <button
              onClick={() => onSelectTab('doctor')}
              className={`w-full flex items-center gap-3 px-3.5 py-2.5 rounded-md text-xs font-semibold tracking-wide uppercase transition-all ${
                currentTab === 'doctor'
                  ? 'app-badge font-bold shadow-sm'
                  : 'app-text-muted hover:app-text-primary app-bg-hover'
              }`}
            >
              <FirstAid size={16} weight={currentTab === 'doctor' ? 'fill' : 'regular'} />
              <span>Doctor</span>
            </button>
          </nav>
        </div>

        {/* Footer Info & Theme Switcher */}
        <div className="p-4 border-t app-border app-bg-main text-xs transition-colors duration-200">
          <div className="flex items-center justify-between mb-3">
            <span className="font-semibold app-text-secondary">Status</span>
            <span className="flex items-center gap-1.5 app-text-primary font-mono text-[11px] font-bold">
              <span className="w-1.5 h-1.5 rounded-full app-badge animate-pulse" />
              ONLINE
            </span>
          </div>

          <button
            onClick={onToggleTheme}
            className="w-full flex items-center justify-between px-3 py-2 rounded border app-border app-bg-card app-text-secondary hover:app-text-primary text-xs font-bold uppercase tracking-wider transition-all"
          >
            <div className="flex items-center gap-2">
              {theme === 'dark' ? <Moon size={16} weight="fill" /> : <Sun size={16} weight="fill" />}
              <span>{theme === 'dark' ? 'Dark Theme' : 'Light Theme'}</span>
            </div>
            <span className="text-[10px] font-mono app-text-muted">{theme.toUpperCase()}</span>
          </button>
        </div>
      </aside>

      {/* Main Content Viewport */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Navbar */}
        <header className="h-16 app-bg-card border-b app-border px-6 flex items-center justify-between flex-shrink-0 transition-colors duration-200">
          <div className="flex items-center gap-3">
            <h2 className="text-sm font-bold app-text-primary uppercase tracking-wider">
              {currentTab === 'pipelines' && 'Pipelines & Execution History'}
              {currentTab === 'build-details' && `Job Output — ${selectedJobId}`}
              {currentTab === 'doctor' && 'System Health Diagnostics'}
            </h2>
          </div>

          <div className="flex items-center gap-3">
            {/* Quick Theme Toggle Icon Button */}
            <button
              onClick={onToggleTheme}
              className="p-2 rounded-md app-bg-main border app-border app-text-secondary hover:app-text-primary transition-colors"
              title={`Switch to ${theme === 'dark' ? 'Light' : 'Dark'} Theme`}
            >
              {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
            </button>

            <button
              onClick={onOpenRunModal}
              className="flex items-center gap-2 app-badge font-bold text-xs px-4 py-2 rounded-md shadow transition-all active:scale-95 uppercase tracking-wide cursor-pointer"
            >
              <Play size={14} weight="fill" />
              <span>Run Pipeline</span>
            </button>
          </div>
        </header>

        {/* Page Content Body */}
        <main className="flex-1 overflow-y-auto p-6 app-bg-main app-text-primary transition-colors duration-200">
          {children}
        </main>
      </div>
    </div>
  );
};
