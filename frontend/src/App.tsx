import { useState, useEffect } from 'react';
import { Layout } from './components/Layout';
import { PipelinesPage } from './pages/Pipelines';
import { BuildDetailsPage } from './pages/BuildDetails';
import { DoctorPage } from './pages/Doctor';
import { RunPipelineModal } from './components/RunPipelineModal';
import { 
  fetchBuildHistory, 
  fetchRunningJobs, 
  triggerBuild, 
} from './api/client';
import type { Job, RunPipelinePayload } from './api/client';

export function App() {
  const [currentTab, setCurrentTab] = useState<'pipelines' | 'build-details' | 'doctor'>('pipelines');
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);

  const [jobs, setJobs] = useState<Job[]>([]);
  const [runningJobs, setRunningJobs] = useState<Job[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isRunModalOpen, setIsRunModalOpen] = useState<boolean>(false);

  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    return (localStorage.getItem('cicd_theme') as 'dark' | 'light') || 'dark';
  });

  const toggleTheme = () => {
    const nextTheme = theme === 'dark' ? 'light' : 'dark';
    setTheme(nextTheme);
    localStorage.setItem('cicd_theme', nextTheme);
  };

  useEffect(() => {
    const root = document.documentElement;
    const body = document.body;
    if (theme === 'light') {
      root.classList.add('light');
      root.classList.remove('dark');
      body.classList.add('light');
      body.classList.remove('dark');
    } else {
      root.classList.add('dark');
      root.classList.remove('light');
      body.classList.add('dark');
      body.classList.remove('light');
    }
  }, [theme]);

  const loadData = async (showLoading = false) => {
    if (showLoading) {
      setIsLoading(true);
    }
    try {
      const [historyData, runningData] = await Promise.all([
        fetchBuildHistory().catch(() => []),
        fetchRunningJobs().catch(() => []),
      ]);
      setJobs(historyData);
      setRunningJobs(runningData);
    } catch (err) {
      console.error('Failed to load dashboard data', err);
    } finally {
      if (showLoading) {
        setIsLoading(false);
      }
    }
  };

  useEffect(() => {
    let isMounted = true;

    Promise.all([
      fetchBuildHistory().catch(() => []),
      fetchRunningJobs().catch(() => []),
    ]).then(([historyData, runningData]) => {
      if (isMounted) {
        setJobs(historyData);
        setRunningJobs(runningData);
        setIsLoading(false);
      }
    });

    // Auto-refresh history every 4 seconds
    const interval = setInterval(() => {
      loadData(false);
    }, 4000);

    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, []);

  const handleSelectTab = (tab: 'pipelines' | 'build-details' | 'doctor', jobId?: string) => {
    setCurrentTab(tab);
    if (tab === 'build-details' && jobId) {
      setSelectedJobId(jobId);
    } else if (tab === 'pipelines') {
      setSelectedJobId(null);
    }
  };

  const handleSelectJob = (jobId: string) => {
    setSelectedJobId(jobId);
    setCurrentTab('build-details');
  };

  const handleRunPipelineSubmit = async (payload: RunPipelinePayload) => {
    const res = await triggerBuild(payload);
    await loadData();
    if (res.job_id) {
      handleSelectJob(res.job_id);
    }
  };

  return (
    <Layout
      currentTab={currentTab}
      selectedJobId={selectedJobId}
      onSelectTab={handleSelectTab}
      onOpenRunModal={() => setIsRunModalOpen(true)}
      activeCount={runningJobs.length}
      theme={theme}
      onToggleTheme={toggleTheme}
    >
      {currentTab === 'pipelines' && (
        <PipelinesPage
          jobs={jobs}
          runningJobs={runningJobs}
          isLoading={isLoading}
          onRefresh={loadData}
          onSelectJob={handleSelectJob}
          onOpenRunModal={() => setIsRunModalOpen(true)}
        />
      )}

      {currentTab === 'build-details' && selectedJobId && (
        <BuildDetailsPage
          jobId={selectedJobId}
          onBack={() => {
            setCurrentTab('pipelines');
            setSelectedJobId(null);
          }}
        />
      )}

      {currentTab === 'doctor' && <DoctorPage />}

      <RunPipelineModal
        isOpen={isRunModalOpen}
        onClose={() => setIsRunModalOpen(false)}
        onSubmit={handleRunPipelineSubmit}
      />
    </Layout>
  );
}

export default App;
