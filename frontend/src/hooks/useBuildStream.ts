import { useState, useEffect, useRef } from 'react';
import type { Job, LogEntry } from '../api/client';
import { fetchJobDetails, fetchJobLogs } from '../api/client';

export interface StreamState {
  job: Job | null;
  logs: LogEntry[];
  isConnected: boolean;
  error: string | null;
}

export function useBuildStream(jobId: string | null): StreamState {
  const [job, setJob] = useState<Job | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!jobId) return;

    let isMounted = true;

    // 1. Fetch initial static REST data for instant rendering
    Promise.all([
      fetchJobDetails(jobId).catch(() => null),
      fetchJobLogs(jobId).catch(() => []),
    ]).then(([initialJob, initialLogs]) => {
      if (isMounted) {
        if (initialJob) setJob(initialJob);
        if (initialLogs && initialLogs.length > 0) {
          setLogs(initialLogs);
        }
      }
    });

    // 2. Initialize EventSource for real-time SSE stream updates
    const url = `/api/builds/${encodeURIComponent(jobId)}/stream`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      if (isMounted) {
        setIsConnected(true);
        setError(null);
      }
    };

    es.addEventListener('log', (event: MessageEvent) => {
      try {
        const entry: LogEntry = JSON.parse(event.data);
        if (!isMounted) return;

        setLogs((prev) => {
          // Avoid duplicate log entries based on timestamp + message
          if (prev.some((l) => l.time === entry.time && l.message === entry.message)) {
            return prev;
          }
          return [...prev, entry];
        });
      } catch (err) {
        console.error('Failed to parse SSE log event', err);
      }
    });

    es.addEventListener('status', (event: MessageEvent) => {
      try {
        const updatedJob: Job = JSON.parse(event.data);
        if (!isMounted) return;
        setJob(updatedJob);
      } catch (err) {
        console.error('Failed to parse SSE status event', err);
      }
    });

    es.onerror = () => {
      if (isMounted) setIsConnected(false);
    };

    return () => {
      isMounted = false;
      es.close();
      eventSourceRef.current = null;
      setIsConnected(false);
    };
  }, [jobId]);

  return { job, logs, isConnected, error };
}
