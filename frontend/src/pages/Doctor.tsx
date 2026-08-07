import React, { useState, useEffect } from 'react';
import { 
  CheckCircle, 
  XCircle, 
  FirstAid, 
  ArrowClockwise, 
  HardDrives, 
  Folder, 
  LockKey,
  Wrench,
  Spinner
} from '@phosphor-icons/react';
import type { DoctorResult } from '../api/client';
import { fetchDoctorDiagnostics } from '../api/client';

export const DoctorPage: React.FC = () => {
  const [result, setResult] = useState<DoctorResult | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadDiagnostics = async (showLoading = false) => {
    if (showLoading) {
      setIsLoading(true);
      setError(null);
    }
    try {
      const data = await fetchDoctorDiagnostics();
      setResult(data);
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to run health diagnostics';
      setError(msg);
    } finally {
      if (showLoading) {
        setIsLoading(false);
      }
    }
  };

  useEffect(() => {
    let isMounted = true;

    fetchDoctorDiagnostics()
      .then((data) => {
        if (isMounted) {
          setResult(data);
          setIsLoading(false);
        }
      })
      .catch((err) => {
        if (isMounted) {
          const msg = err instanceof Error ? err.message : 'Failed to run health diagnostics';
          setError(msg);
          setIsLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Doctor Header */}
      <div className="app-bg-card border app-border rounded-lg p-6 shadow-xl flex items-center justify-between transition-colors">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded app-badge flex items-center justify-center font-bold">
            <FirstAid size={26} weight="fill" />
          </div>
          <div>
            <h2 className="text-base font-bold app-text-primary uppercase tracking-wider">System Health Doctor</h2>
            <p className="text-xs app-text-muted mt-0.5 font-mono">
              Automated diagnostics verifying build tools, permissions, and storage.
            </p>
          </div>
        </div>

        <button
          onClick={() => loadDiagnostics(true)}
          disabled={isLoading}
          className="flex items-center gap-2 app-bg-main border app-border hover:border-current app-text-primary text-xs font-bold uppercase tracking-wider px-4 py-2 rounded transition-colors cursor-pointer"
        >
          <ArrowClockwise size={16} className={isLoading ? 'animate-spin' : ''} />
          <span>Re-Run Diagnostics</span>
        </button>
      </div>

      {error && (
        <div className="p-4 rounded app-bg-card border app-border app-text-primary text-xs font-mono flex items-center gap-3">
          <XCircle size={20} weight="fill" />
          <span>{error}</span>
        </div>
      )}

      {/* System Overall Status */}
      {result && (
        <div className="p-4 rounded border app-bg-card app-border flex items-center justify-between app-text-primary">
          <div className="flex items-center gap-3">
            {result.all_ok ? (
              <CheckCircle size={24} weight="fill" className="app-text-primary" />
            ) : (
              <XCircle size={24} weight="fill" className="app-text-muted" />
            )}
            <div>
              <span className="font-bold text-xs uppercase tracking-wider">
                {result.all_ok ? 'All System Checks Passed' : 'System Configuration Warnings Found'}
              </span>
              <p className="text-xs app-text-muted font-mono mt-0.5">
                {result.all_ok
                  ? 'All required compilers, toolchains, and workspace paths are healthy.'
                  : 'Check missing tools or permission issues below.'}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Required Build Tools Grid */}
      <div className="space-y-3">
        <h3 className="text-[11px] font-bold app-text-muted uppercase tracking-widest">
          Compilers & Execution Toolchains
        </h3>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {[
            { key: 'svn', label: 'Subversion (SVN)', desc: 'Source control checkout engine' },
            { key: 'node', label: 'Node.js Runtime', desc: 'JavaScript build environment' },
            { key: 'npm', label: 'NPM Package Manager', desc: 'Node package resolution' },
            { key: 'dotnet', label: '.NET Core SDK', desc: '.NET solution builder & restorer' },
          ].map((tool) => {
            const isOK = result?.tools?.[tool.key] ?? false;
            return (
              <div
                key={tool.key}
                className="app-bg-card border app-border rounded p-4 flex items-center justify-between transition-colors"
              >
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 rounded app-bg-main border app-border flex items-center justify-center app-text-primary">
                    <Wrench size={18} />
                  </div>
                  <div>
                    <h4 className="text-xs font-bold app-text-primary uppercase tracking-wider">{tool.label}</h4>
                    <p className="text-[11px] app-text-muted font-mono">{tool.desc}</p>
                  </div>
                </div>

                {isLoading ? (
                  <Spinner size={18} className="animate-spin app-text-muted" />
                ) : isOK ? (
                  <span className="inline-flex items-center gap-1 text-xs font-bold app-badge px-2.5 py-1 rounded uppercase tracking-wider">
                    <CheckCircle size={14} weight="fill" />
                    Available
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 text-xs font-bold app-text-muted app-bg-main border app-border px-2.5 py-1 rounded uppercase tracking-wider">
                    <XCircle size={14} weight="fill" />
                    Not Found
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Permissions & Directory Checks */}
      <div className="space-y-3 pt-2">
        <h3 className="text-[11px] font-bold app-text-muted uppercase tracking-widest">
          Directories & Storage Health
        </h3>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="app-bg-card border app-border rounded p-4 flex items-center justify-between transition-colors">
            <div className="flex items-center gap-2.5">
              <Folder size={18} className="app-text-primary" />
              <span className="text-xs font-bold app-text-secondary uppercase tracking-wider">Workspace Root</span>
            </div>
            {result?.workspace_ok ? (
              <CheckCircle size={18} weight="fill" className="app-text-primary" />
            ) : (
              <XCircle size={18} weight="fill" className="app-text-muted" />
            )}
          </div>

          <div className="app-bg-card border app-border rounded p-4 flex items-center justify-between transition-colors">
            <div className="flex items-center gap-2.5">
              <HardDrives size={18} className="app-text-primary" />
              <span className="text-xs font-bold app-text-secondary uppercase tracking-wider">Storage Dir</span>
            </div>
            {result?.storage_ok ? (
              <CheckCircle size={18} weight="fill" className="app-text-primary" />
            ) : (
              <XCircle size={18} weight="fill" className="app-text-muted" />
            )}
          </div>

          <div className="app-bg-card border app-border rounded p-4 flex items-center justify-between transition-colors">
            <div className="flex items-center gap-2.5">
              <LockKey size={18} className="app-text-primary" />
              <span className="text-xs font-bold app-text-secondary uppercase tracking-wider">Credentials</span>
            </div>
            {result?.credentials_ok ? (
              <CheckCircle size={18} weight="fill" className="app-text-primary" />
            ) : (
              <XCircle size={18} weight="fill" className="app-text-muted" />
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
