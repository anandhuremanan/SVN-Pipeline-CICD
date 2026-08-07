import React, { useState } from 'react';
import { 
  CheckCircle, 
  XCircle, 
  Spinner, 
  Clock, 
  GitBranch, 
  MagnifyingGlass,
  ArrowClockwise,
  Play,
  Cube,
  Circle
} from '@phosphor-icons/react';
import type { Job } from '../api/client';

interface PipelinesPageProps {
  jobs: Job[];
  runningJobs: Job[];
  isLoading: boolean;
  onRefresh: () => void;
  onSelectJob: (jobId: string) => void;
  onOpenRunModal: () => void;
}

export const PipelinesPage: React.FC<PipelinesPageProps> = ({
  jobs,
  runningJobs,
  isLoading,
  onRefresh,
  onSelectJob,
  onOpenRunModal,
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [filterStatus, setFilterStatus] = useState<string>('ALL');

  // Combine running + historical jobs (avoiding duplicate IDs and null inputs)
  const safeRunningJobs = Array.isArray(runningJobs) ? runningJobs : [];
  const safeJobs = Array.isArray(jobs) ? jobs : [];

  const allJobsMap = new Map<string, Job>();
  safeRunningJobs.forEach((j) => {
    if (j && j.id) allJobsMap.set(j.id, j);
  });
  safeJobs.forEach((j) => {
    if (j && j.id && !allJobsMap.has(j.id)) allJobsMap.set(j.id, j);
  });
  const allJobs = Array.from(allJobsMap.values());

  const totalBuilds = allJobs.length;
  const successCount = allJobs.filter((j) => j.status === 'SUCCESS').length;
  const failedCount = allJobs.filter((j) => j.status === 'FAILED').length;
  const activeCount = runningJobs.length;

  const filteredJobs = allJobs.filter((j) => {
    const matchesSearch = 
      j.id.toLowerCase().includes(searchTerm.toLowerCase()) ||
      j.repository.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (j.revision && j.revision.toLowerCase().includes(searchTerm.toLowerCase()));

    const matchesStatus = 
      filterStatus === 'ALL' || j.status === filterStatus;

    return matchesSearch && matchesStatus;
  });

  const formatDuration = (nanoseconds: number) => {
    if (!nanoseconds) return '0s';
    const seconds = Math.floor(nanoseconds / 1e9);
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remSeconds = seconds % 60;
    return `${minutes}m ${remSeconds}s`;
  };

  const getStatusBadge = (status: Job['status']) => {
    switch (status) {
      case 'SUCCESS':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-bold app-badge tracking-wider uppercase border border-current">
            <CheckCircle size={14} weight="fill" />
            Succeeded
          </span>
        );
      case 'FAILED':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-bold app-bg-card app-text-primary border app-border tracking-wider uppercase">
            <XCircle size={14} weight="fill" />
            Failed
          </span>
        );
      case 'RUNNING':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-bold app-bg-card app-text-primary border app-border tracking-wider uppercase animate-pulse-monochrome">
            <Spinner size={14} className="animate-spin" />
            Running
          </span>
        );
      case 'INTERRUPTED':
      case 'CANCELLED':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-semibold app-bg-main app-text-muted border app-border tracking-wider uppercase">
            <Circle size={14} />
            Interrupted
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-semibold app-bg-main app-text-muted border app-border tracking-wider uppercase">
            Pending
          </span>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* KPI Stats Cards Header */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="app-bg-card border app-border rounded-lg p-5 transition-colors">
          <p className="text-[11px] font-bold app-text-muted uppercase tracking-widest">Total Builds</p>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-3xl font-bold app-text-primary font-mono">{totalBuilds}</span>
            <span className="text-xs app-text-muted font-mono">Executions</span>
          </div>
        </div>

        <div className="app-bg-card border app-border rounded-lg p-5 transition-colors">
          <p className="text-[11px] font-bold app-text-muted uppercase tracking-widest">Successful</p>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-3xl font-bold app-text-primary font-mono">{successCount}</span>
            <span className="text-xs app-text-muted font-mono">
              {totalBuilds > 0 ? `${Math.round((successCount / totalBuilds) * 100)}%` : '0%'}
            </span>
          </div>
        </div>

        <div className="app-bg-card border app-border rounded-lg p-5 transition-colors">
          <p className="text-[11px] font-bold app-text-muted uppercase tracking-widest">Failed</p>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-3xl font-bold app-text-secondary font-mono">{failedCount}</span>
            <span className="text-xs app-text-muted font-mono">
              {totalBuilds > 0 ? `${Math.round((failedCount / totalBuilds) * 100)}%` : '0%'}
            </span>
          </div>
        </div>

        <div className="app-bg-card border app-border rounded-lg p-5 transition-colors">
          <p className="text-[11px] font-bold app-text-muted uppercase tracking-widest">Active Workers</p>
          <div className="mt-2 flex items-baseline justify-between">
            <span className="text-3xl font-bold app-text-primary font-mono">{activeCount}</span>
            <span className="text-xs app-text-muted font-mono">Running</span>
          </div>
        </div>
      </div>

      {/* Filter and Control Bar */}
      <div className="app-bg-card border app-border rounded-lg p-4 flex flex-col sm:flex-row items-center justify-between gap-4 transition-colors">
        <div className="flex items-center gap-3 w-full sm:w-auto flex-1">
          <div className="relative w-full sm:max-w-xs">
            <MagnifyingGlass size={16} className="absolute left-3 top-3 app-text-muted" />
            <input
              type="text"
              placeholder="Search repository or Job ID..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full app-bg-input border app-border rounded pl-9 pr-3.5 py-2 text-xs app-text-primary placeholder:app-text-muted focus:outline-none font-mono"
            />
          </div>

          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary focus:outline-none uppercase font-mono"
          >
            <option value="ALL">All Statuses</option>
            <option value="SUCCESS">Succeeded</option>
            <option value="FAILED">Failed</option>
            <option value="RUNNING">Running</option>
          </select>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={onRefresh}
            className="p-2 app-bg-input border app-border hover:border-current rounded app-text-muted hover:app-text-primary transition-colors cursor-pointer"
            title="Refresh history"
          >
            <ArrowClockwise size={16} className={isLoading ? 'animate-spin' : ''} />
          </button>
        </div>
      </div>

      {/* Pipelines History Table */}
      <div className="app-bg-card border app-border rounded-lg overflow-hidden shadow-2xl transition-colors">
        {filteredJobs.length === 0 ? (
          <div className="p-12 text-center">
            <Cube size={40} className="mx-auto app-text-muted mb-3" />
            <p className="text-sm font-bold app-text-primary uppercase tracking-wider">No pipeline builds found</p>
            <p className="text-xs app-text-muted mt-1">Run a new SVN pipeline to view live execution metrics here.</p>
            <button
              onClick={onOpenRunModal}
              className="mt-4 inline-flex items-center gap-2 app-badge font-bold text-xs px-4 py-2 rounded uppercase tracking-wider cursor-pointer"
            >
              <Play size={14} weight="fill" />
              <span>Run Pipeline Now</span>
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="app-table-header border-b app-border text-[11px] font-bold app-text-muted uppercase tracking-widest">
                  <th className="py-3.5 px-5">Status</th>
                  <th className="py-3.5 px-5">Job ID</th>
                  <th className="py-3.5 px-5">Repository URL</th>
                  <th className="py-3.5 px-5">Revision</th>
                  <th className="py-3.5 px-5">Started At</th>
                  <th className="py-3.5 px-5 text-right">Duration</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800 dark:divide-zinc-800 light:divide-zinc-200 text-xs">
                {filteredJobs.map((job) => (
                  <tr
                    key={job.id}
                    onClick={() => onSelectJob(job.id)}
                    className="app-bg-hover cursor-pointer transition-colors group"
                  >
                    <td className="py-4 px-5 whitespace-nowrap">
                      {getStatusBadge(job.status)}
                    </td>
                    <td className="py-4 px-5 font-mono app-text-primary font-bold group-hover:underline whitespace-nowrap">
                      {job.id}
                    </td>
                    <td className="py-4 px-5 app-text-secondary font-mono max-w-xs truncate">
                      <div className="flex items-center gap-2">
                        <GitBranch size={15} className="app-text-muted flex-shrink-0" />
                        <span className="truncate">{job.repository || 'SVN Pipeline'}</span>
                      </div>
                    </td>
                    <td className="py-4 px-5 app-text-muted font-mono whitespace-nowrap">
                      r{job.revision || 'HEAD'}
                    </td>
                    <td className="py-4 px-5 app-text-muted whitespace-nowrap font-mono text-[11px]">
                      {job.started_at ? new Date(job.started_at).toLocaleString() : 'N/A'}
                    </td>
                    <td className="py-4 px-5 text-right font-mono app-text-primary whitespace-nowrap font-bold">
                      <div className="flex items-center justify-end gap-1.5">
                        <Clock size={14} className="app-text-muted" />
                        <span>{formatDuration(job.duration)}</span>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};
