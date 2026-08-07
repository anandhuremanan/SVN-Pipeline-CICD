import React, { useState, useEffect, useRef } from 'react';
import { 
  CheckCircle, 
  XCircle, 
  Spinner, 
  Clock, 
  Terminal, 
  Copy, 
  ArrowDown, 
  FolderOpen,
  GitBranch,
  ArrowLeft,
  CalendarCheck,
  Circle,
  CaretRight,
  ListChecks
} from '@phosphor-icons/react';
import { useBuildStream } from '../hooks/useBuildStream';

interface BuildDetailsPageProps {
  jobId: string;
  onBack: () => void;
}

interface AzureStep {
  id: string;
  name: string;
  category: 'setup' | 'checkout' | 'scan' | 'build' | 'deploy' | 'cleanup';
  project?: string;
}

export const BuildDetailsPage: React.FC<BuildDetailsPageProps> = ({ jobId, onBack }) => {
  const { job: liveJob, logs: liveLogs, isConnected } = useBuildStream(jobId);
  const [selectedStepFilter, setSelectedStepFilter] = useState<string | null>(null);
  const [logFilter, setLogFilter] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const terminalEndRef = useRef<HTMLDivElement>(null);

  const currentJob = liveJob;
  const logs = liveLogs;

  useEffect(() => {
    if (autoScroll && terminalEndRef.current) {
      terminalEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  const formatDuration = (nanoseconds?: number) => {
    if (!nanoseconds) return '0s';
    const seconds = Math.floor(nanoseconds / 1e9);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remSeconds = seconds % 60;
    return `${minutes}m ${remSeconds}s`;
  };

  // Dynamically resolve target projects from job or log stream
  const discoveredTargets = Array.from(
    new Set([
      ...(currentJob?.build_results || []).map((r) => r.project.name),
      ...logs.map((l) => l.project).filter((p): p is string => Boolean(p)),
      'Client',
      'API',
    ])
  );

  // Define standard Azure DevOps Pipeline Execution Steps
  const azureSteps: AzureStep[] = [
    { id: 'env', name: 'Initialize Job & Environment', category: 'setup' },
    { id: 'checkout', name: 'Checkout SVN Repository Source', category: 'checkout' },
    { id: 'scan', name: 'Scan Solution & Discover Targets', category: 'scan' },
    ...discoveredTargets.map((t) => ({
      id: `build-${t}`,
      name: `Build Target: ${t}`,
      category: 'build' as const,
      project: t,
    })),
    { id: 'deploy', name: 'Publish Output & Artifacts', category: 'deploy' },
    { id: 'cleanup', name: 'Post-Job Workspace Cleanup', category: 'cleanup' },
  ];

  const hasLogMessage = (keyword: string) =>
    logs.some((l) => l.message && l.message.toLowerCase().includes(keyword.toLowerCase()));

  const hasProjectLog = (proj: string) =>
    logs.some((l) => l.project === proj);

  const hasProjectError = (proj: string) =>
    logs.some((l) => l.project === proj && l.level === 'ERROR');

  const hasProjectSuccess = (proj: string) =>
    logs.some((l) => l.project === proj && (l.message.includes('completed') || l.message.includes('exit code 0')));

  // Real-time Azure DevOps Step Status Evaluator
  const getStepStatus = (step: AzureStep) => {
    if (!currentJob) return 'pending';

    // 1. Final Job Status Overrides
    if (currentJob.status === 'SUCCESS') {
      return 'success';
    }

    if (currentJob.status === 'FAILED') {
      if (step.project && hasProjectError(step.project)) {
        return 'failed';
      }
      if (step.category === 'deploy' && hasLogMessage('publishing')) {
        return 'failed';
      }
      if (step.category === 'setup' && (hasLogMessage('checking environment') || hasLogMessage('workspace'))) {
        return 'success';
      }
      if (step.category === 'checkout' && hasLogMessage('checkout')) {
        return 'success';
      }
      if (step.category === 'scan' && hasLogMessage('scanning')) {
        return 'success';
      }
      if (step.project && hasProjectSuccess(step.project)) {
        return 'success';
      }
      if (hasLogMessage('failed') || hasLogMessage('error')) return 'failed';
      return 'success';
    }

    // 2. Dynamic Real-time RUNNING status evaluation via live log stream
    if (currentJob.status === 'RUNNING') {
      const hasCheckout = hasLogMessage('checkout') || hasLogMessage('created isolated workspace');
      const hasScan = hasLogMessage('scanning') || hasLogMessage('targets') || hasLogMessage('executing build plan');
      const hasPublish = hasLogMessage('publishing') || hasLogMessage('deployable');
      const hasCleanup = hasLogMessage('retained') || hasLogMessage('cleaning');

      if (step.category === 'setup') {
        return hasCheckout ? 'success' : 'running';
      }
      if (step.category === 'checkout') {
        if (hasScan) return 'success';
        return hasCheckout ? 'running' : 'pending';
      }
      if (step.category === 'scan') {
        if (hasLogMessage('executing step') || (step.project && hasProjectLog(step.project))) return 'success';
        return hasScan ? 'running' : 'pending';
      }
      if (step.category === 'build') {
        if (hasPublish || hasCleanup) return 'success';
        if (step.project && hasProjectSuccess(step.project)) return 'success';
        if (step.project && hasProjectLog(step.project)) return 'running';
        if (hasScan) return 'running';
        return 'pending';
      }
      if (step.category === 'deploy') {
        if (hasCleanup) return 'success';
        return hasPublish ? 'running' : 'pending';
      }
      if (step.category === 'cleanup') {
        return hasCleanup ? 'running' : 'pending';
      }
    }

    return 'pending';
  };

  const filteredLogs = logs.filter((l) => {
    const matchesText = 
      l.message.toLowerCase().includes(logFilter.toLowerCase()) ||
      (l.step && l.step.toLowerCase().includes(logFilter.toLowerCase())) ||
      (l.project && l.project.toLowerCase().includes(logFilter.toLowerCase()));

    if (!matchesText) return false;

    if (selectedStepFilter) {
      const stepObj = azureSteps.find((s) => s.id === selectedStepFilter);
      if (stepObj?.project) {
        return l.project === stepObj.project;
      }
      return true;
    }

    return true;
  });

  const copyAllLogs = () => {
    const text = logs.map((l) => `[${l.time}] [${l.level}] ${l.message}`).join('\n');
    navigator.clipboard.writeText(text);
  };

  return (
    <div className="space-y-6">
      {/* Back Button & Header */}
      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-2 text-xs font-bold app-text-secondary hover:app-text-primary uppercase tracking-wider app-bg-card border app-border px-3.5 py-2 rounded transition-colors cursor-pointer"
        >
          <ArrowLeft size={16} />
          <span>Back to Pipelines</span>
        </button>

        <div className="flex items-center gap-2">
          {isConnected ? (
            <span className="inline-flex items-center gap-1.5 text-xs font-mono font-bold app-text-primary app-bg-card border app-border px-3 py-1 rounded-full uppercase">
              <span className="w-2 h-2 rounded-full app-badge animate-pulse" />
              Live SSE Active
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 text-xs font-mono font-bold app-text-muted app-bg-card border app-border px-3 py-1 rounded-full uppercase">
              Completed Log View
            </span>
          )}
        </div>
      </div>

      {/* Main Overview Card */}
      <div className="app-bg-card border app-border rounded-lg p-6 shadow-xl transition-colors">
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 border-b app-border pb-5">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-xl font-bold app-text-primary font-mono">{jobId}</h2>
              {currentJob?.status === 'SUCCESS' && (
                <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded text-xs font-bold app-badge uppercase tracking-wider">
                  <CheckCircle size={16} weight="fill" />
                  Succeeded
                </span>
              )}
              {currentJob?.status === 'FAILED' && (
                <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded text-xs font-bold app-bg-main app-text-primary border app-border uppercase tracking-wider">
                  <XCircle size={16} weight="fill" />
                  Failed
                </span>
              )}
              {currentJob?.status === 'RUNNING' && (
                <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded text-xs font-bold app-bg-main app-text-primary border app-border uppercase tracking-wider animate-pulse-monochrome">
                  <Spinner size={16} className="animate-spin" />
                  Running
                </span>
              )}
            </div>
            <p className="text-xs app-text-muted mt-1.5 flex items-center gap-2">
              <GitBranch size={14} className="app-text-muted" />
              <span className="font-mono app-text-secondary">{currentJob?.repository || 'SVN Repository'}</span>
              <span>•</span>
              <span className="font-mono">r{currentJob?.revision || 'HEAD'}</span>
            </p>
          </div>

          <div className="flex items-center gap-4 text-xs app-text-primary app-bg-main px-4 py-2.5 rounded border app-border font-mono font-bold">
            <div className="flex items-center gap-1.5">
              <Clock size={16} className="app-text-muted" />
              <span>{formatDuration(currentJob?.duration)}</span>
            </div>
            <span className="app-text-muted">|</span>
            <div className="flex items-center gap-1.5">
              <CalendarCheck size={16} className="app-text-muted" />
              <span>{currentJob?.started_at ? new Date(currentJob.started_at).toLocaleTimeString() : 'N/A'}</span>
            </div>
          </div>
        </div>

        {/* Retained Workspace Location Info */}
        {currentJob?.workspace && (
          <div className="mt-4 pt-2 flex items-center justify-between text-xs app-text-muted">
            <div className="flex items-center gap-2">
              <FolderOpen size={16} className="app-text-muted" />
              <span>Workspace:</span>
              <span className="font-mono app-text-secondary">{currentJob.workspace}</span>
            </div>
            <span className="text-[11px] app-text-primary font-mono font-bold uppercase">
              {currentJob.workspace_retained ? 'Retained on disk' : 'Cleaned'}
            </span>
          </div>
        )}

        {/* Published Target Destinations Card */}
        {currentJob?.deployment_outputs && Object.keys(currentJob.deployment_outputs).length > 0 && (
          <div className="mt-4 pt-4 border-t app-border">
            <h3 className="text-[11px] font-bold app-text-muted uppercase tracking-widest mb-3">
              Published Deployment Destinations
            </h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {Object.entries(currentJob.deployment_outputs).map(([proj, destPath]) => (
                <div key={proj} className="app-bg-main border app-border rounded p-3 flex items-center justify-between">
                  <div className="flex items-center gap-2.5 truncate">
                    <FolderOpen size={18} className="app-text-primary flex-shrink-0" />
                    <div className="truncate">
                      <span className="block text-[11px] font-bold app-text-primary uppercase">{proj}</span>
                      <span className="block text-[11px] font-mono app-text-secondary truncate">{destPath}</span>
                    </div>
                  </div>
                  <span className="text-[10px] font-mono app-badge px-2 py-0.5 rounded font-bold uppercase flex-shrink-0 ml-2">
                    Published
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Azure DevOps Execution View Split: Steps Timeline (Left) + Console Terminal (Right) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Column: Azure DevOps Pipeline Execution Steps Timeline */}
        <div className="app-bg-card border app-border rounded-lg p-5 space-y-4 shadow-xl transition-colors">
          <div className="flex items-center justify-between border-b app-border pb-3">
            <div className="flex items-center gap-2 text-xs font-bold app-text-primary uppercase tracking-wider">
              <ListChecks size={18} />
              <span>Execution Steps Timeline</span>
            </div>
            <button
              onClick={() => setSelectedStepFilter(null)}
              className={`text-[10px] font-mono px-2 py-0.5 rounded cursor-pointer ${
                !selectedStepFilter ? 'app-badge font-bold' : 'app-text-muted hover:app-text-primary'
              }`}
            >
              All Logs
            </button>
          </div>

          <div className="space-y-2">
            {azureSteps.map((step) => {
              const status = getStepStatus(step);
              const isSelected = selectedStepFilter === step.id;

              return (
                <div
                  key={step.id}
                  onClick={() => setSelectedStepFilter(isSelected ? null : step.id)}
                  className={`p-3 rounded-lg border transition-all cursor-pointer flex items-center justify-between ${
                    isSelected
                      ? 'app-bg-main border-current shadow-md'
                      : 'app-bg-main border-transparent hover:app-border'
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="flex-shrink-0">
                      {status === 'success' && (
                        <CheckCircle size={18} weight="fill" className="app-text-primary" />
                      )}
                      {status === 'running' && (
                        <Spinner size={18} className="animate-spin app-text-primary" />
                      )}
                      {status === 'failed' && (
                        <XCircle size={18} weight="fill" className="app-text-secondary" />
                      )}
                      {status === 'pending' && (
                        <Circle size={18} className="app-text-muted" />
                      )}
                    </div>

                    <div className="truncate">
                      <h4 className="text-xs font-bold app-text-primary uppercase tracking-wide truncate">
                        {step.name}
                      </h4>
                      {step.project && (
                        <p className="text-[10px] app-text-muted font-mono mt-0.5">
                          Target: {step.project}
                        </p>
                      )}
                    </div>
                  </div>

                  <CaretRight size={14} className="app-text-muted flex-shrink-0" />
                </div>
              );
            })}
          </div>
        </div>

        {/* Right Column: Console Execution Log Terminal */}
        <div className="lg:col-span-2 app-bg-main border app-border rounded-lg overflow-hidden shadow-2xl flex flex-col h-[560px] transition-colors">
          {/* Terminal Header Bar */}
          <div className="h-12 app-bg-card border-b app-border px-4 flex items-center justify-between">
            <div className="flex items-center gap-2 text-xs font-mono font-bold app-text-primary">
              <Terminal size={18} className="app-text-primary" />
              <span className="uppercase tracking-wider">Console Output</span>
              <span className="app-text-muted">({filteredLogs.length} lines)</span>
            </div>

            <div className="flex items-center gap-3">
              <input
                type="text"
                placeholder="Filter logs..."
                value={logFilter}
                onChange={(e) => setLogFilter(e.target.value)}
                className="app-bg-input border app-border rounded px-2.5 py-1 text-xs app-text-primary placeholder:app-text-muted focus:outline-none font-mono w-36"
              />

              <button
                onClick={() => setAutoScroll(!autoScroll)}
                className={`p-1.5 rounded text-xs font-mono flex items-center gap-1 uppercase transition-colors cursor-pointer ${
                  autoScroll ? 'app-badge font-bold' : 'app-text-muted hover:app-text-primary'
                }`}
                title="Toggle Auto-Scroll"
              >
                <ArrowDown size={14} />
                <span className="hidden sm:inline">Auto-Scroll</span>
              </button>

              <button
                onClick={copyAllLogs}
                className="p-1.5 app-text-muted hover:app-text-primary app-bg-hover rounded transition-colors cursor-pointer"
                title="Copy all logs"
              >
                <Copy size={16} />
              </button>
            </div>
          </div>

          {/* Terminal Logs Viewport */}
          <div className="flex-1 p-4 font-mono text-[12px] leading-relaxed overflow-y-auto app-bg-main app-text-primary space-y-1">
            {filteredLogs.length === 0 ? (
              <div className="h-full flex items-center justify-center app-text-muted uppercase tracking-widest text-xs">
                <span>Waiting for console log output...</span>
              </div>
            ) : (
              filteredLogs.map((log, idx) => (
                <div key={idx} className="flex items-start gap-3 app-bg-hover px-1 py-0.5 rounded">
                  <span className="app-text-muted select-none text-[11px] w-20 flex-shrink-0">
                    {log.time ? new Date(log.time).toLocaleTimeString() : ''}
                  </span>
                  <span
                    className={`text-[10px] px-1.5 py-0.2 rounded uppercase font-bold tracking-wider flex-shrink-0 ${
                      log.level === 'ERROR'
                        ? 'app-badge'
                        : log.level === 'WARN'
                        ? 'app-bg-card app-text-primary border app-border'
                        : 'app-bg-main app-text-secondary border app-border'
                    }`}
                  >
                    {log.level || 'INFO'}
                  </span>
                  {log.project && (
                    <span className="app-text-primary font-bold flex-shrink-0">[{log.project}]</span>
                  )}
                  <span className="whitespace-pre-wrap break-all app-text-primary">{log.message}</span>
                </div>
              ))
            )}
            <div ref={terminalEndRef} />
          </div>
        </div>
      </div>
    </div>
  );
};
