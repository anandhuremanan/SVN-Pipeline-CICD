import React, { useState, useMemo } from 'react';
import { X, Play, FileCode, Link, Warning, Plus, Trash, Code, Gear, Stack, HardDrives } from '@phosphor-icons/react';
import type { RunPipelinePayload } from '../api/client';

interface ProjectFormItem {
  name: string;
  path: string;
  output: string;
  buildScript?: string;
  isTarget: boolean;
}

interface RunPipelineModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (payload: RunPipelinePayload) => Promise<void>;
}

export const RunPipelineModal: React.FC<RunPipelineModalProps> = ({
  isOpen,
  onClose,
  onSubmit,
}) => {
  const [mode, setMode] = useState<'visual' | 'raw' | 'url'>('visual');

  // Simple URL mode state
  const [repositoryUrl, setRepositoryUrl] = useState('https://versionvault.com');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [revision, setRevision] = useState('HEAD');
  const [keepWorkspace, setKeepWorkspace] = useState(false);

  // Visual YAML Builder state (Supporting ALL YAML Options)
  const [repoType, setRepoType] = useState<'svn'>('svn');
  const [buildStrategy, setBuildStrategy] = useState<'fail-fast' | 'continue'>('fail-fast');
  const [replaceExisting, setReplaceExisting] = useState<boolean>(true);
  const [publishPolicy, setPublishPolicy] = useState<'all-or-nothing' | 'successful-only'>('all-or-nothing');

  const [projects, setProjects] = useState<ProjectFormItem[]>([
    {
      name: 'Client',
      path: 'Client',
      output: 'D:\\svn_ci_cd_test\\TPP\\Client',
      buildScript: '',
      isTarget: true,
    },
    {
      name: 'API',
      path: 'Api',
      output: 'D:\\svn_ci_cd_test\\TPP\\API',
      buildScript: '',
      isTarget: true,
    },
  ]);

  // Raw YAML Custom Override state
  const [customRawYaml, setCustomRawYaml] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Generate dynamic YAML string directly during render using useMemo
  const generatedYaml = useMemo(() => {
    const activeTargets = projects.filter((p) => p.isTarget).map((p) => p.name);

    let yaml = `version: 1\n\n`;
    yaml += `# Repository details\n`;
    yaml += `repository:\n`;
    yaml += `  type: ${repoType}\n`;
    yaml += `  url: ${repositoryUrl || 'https://versionvault.com'}\n\n`;

    yaml += `# Build strategy & target ordering\n`;
    yaml += `build:\n`;
    yaml += `  strategy: ${buildStrategy}\n`;
    if (activeTargets.length > 0) {
      yaml += `  targets:\n`;
      activeTargets.forEach((t) => {
        yaml += `    - ${t}\n`;
      });
    }
    yaml += `\n`;

    yaml += `# Output publishing settings\n`;
    yaml += `deployment:\n`;
    yaml += `  replaceExisting: ${replaceExisting}\n`;
    yaml += `  publish: ${publishPolicy}\n\n`;

    yaml += `# List of projects inside the repository\n`;
    yaml += `projects:\n`;
    projects.forEach((p) => {
      yaml += `  - name: ${p.name || 'Project'}\n`;
      yaml += `    path: ${p.path || '""'}\n`;
      yaml += `    output: ${p.output || 'D:\\\\Deployments'}\n`;
      if (p.buildScript && p.buildScript.trim().length > 0) {
        yaml += `    build:\n`;
        yaml += `      script: ${p.buildScript.trim()}\n`;
      }
      yaml += `\n`;
    });

    return yaml;
  }, [repositoryUrl, repoType, buildStrategy, replaceExisting, publishPolicy, projects]);

  const activeYamlContent = mode === 'visual' ? generatedYaml : (customRawYaml ?? generatedYaml);

  if (!isOpen) return null;

  const handleAddProject = () => {
    setProjects((prev) => [
      ...prev,
      {
        name: `Project${prev.length + 1}`,
        path: `Project${prev.length + 1}`,
        output: `D:\\Deployments\\Project${prev.length + 1}`,
        buildScript: '',
        isTarget: true,
      },
    ]);
  };

  const handleRemoveProject = (index: number) => {
    setProjects((prev) => prev.filter((_, i) => i !== index));
  };

  const handleUpdateProject = <K extends keyof ProjectFormItem>(
    index: number,
    field: K,
    value: ProjectFormItem[K]
  ) => {
    setProjects((prev) =>
      prev.map((p, i) => (i === index ? { ...p, [field]: value } : p))
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitError(null);

    setIsSubmitting(true);
    try {
      const payload: RunPipelinePayload = {
        revision: revision.trim() || 'HEAD',
        keep_workspace: keepWorkspace,
        username: username.trim() || undefined,
        password: password.trim() || undefined,
      };

      if (repositoryUrl.trim()) {
        payload.repository_url = repositoryUrl.trim();
      }

      if (mode === 'url') {
        if (!repositoryUrl.trim()) {
          throw new Error('SVN Repository URL is required.');
        }
      } else {
        const yamlToUse = activeYamlContent;
        if (!yamlToUse.trim()) {
          throw new Error('YAML content cannot be empty.');
        }
        payload.pipeline_content = yamlToUse;
      }

      await onSubmit(payload);
      onClose();
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to start build job';
      setSubmitError(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md">
      <div className="w-full max-w-4xl app-bg-card border app-border rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh] transition-colors">
        {/* Modal Header */}
        <div className="px-6 py-4 border-b app-border flex items-center justify-between app-bg-main flex-shrink-0">
          <div className="flex items-center gap-3 app-text-primary font-bold text-sm tracking-wider uppercase">
            <div className="w-8 h-8 rounded app-badge flex items-center justify-center">
              <Play size={16} weight="fill" />
            </div>
            <div>
              <h2>Run Pipeline & YAML Configurator</h2>
              <p className="text-[10px] app-text-muted font-mono font-normal uppercase">
                Full Visual YAML Schema Options & Target Resolution
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="app-text-muted hover:app-text-primary p-1 rounded transition-colors cursor-pointer"
          >
            <X size={20} />
          </button>
        </div>

        {/* Mode Selector Header Bar */}
        <div className="px-6 py-3 app-bg-card border-b app-border flex items-center justify-between flex-shrink-0">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setMode('visual')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-xs font-bold uppercase tracking-wider transition-all cursor-pointer ${
                mode === 'visual' ? 'app-badge' : 'app-text-muted hover:app-text-primary'
              }`}
            >
              <Gear size={14} />
              <span>Visual Builder</span>
            </button>

            <button
              type="button"
              onClick={() => setMode('raw')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-xs font-bold uppercase tracking-wider transition-all cursor-pointer ${
                mode === 'raw' ? 'app-badge' : 'app-text-muted hover:app-text-primary'
              }`}
            >
              <Code size={14} />
              <span>Raw YAML Editor</span>
            </button>

            <button
              type="button"
              onClick={() => setMode('url')}
              className={`flex items-center gap-2 px-3 py-1.5 rounded text-xs font-bold uppercase tracking-wider transition-all cursor-pointer ${
                mode === 'url' ? 'app-badge' : 'app-text-muted hover:app-text-primary'
              }`}
            >
              <Link size={14} />
              <span>Simple SVN URL</span>
            </button>
          </div>
        </div>

        {/* Modal Scrollable Form Area */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
          {submitError && (
            <div className="p-3.5 rounded app-bg-main border app-border app-text-primary text-xs font-mono flex items-start gap-2">
              <Warning size={18} className="flex-shrink-0 mt-0.5" />
              <span>{submitError}</span>
            </div>
          )}

          {/* MODE 1: VISUAL YAML BUILDER */}
          {mode === 'visual' && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              {/* Form Controls Column */}
              <div className="space-y-5">
                {/* Repository Settings */}
                <div className="app-bg-main border app-border rounded-lg p-4 space-y-3">
                  <div className="flex items-center gap-2 text-xs font-bold app-text-primary uppercase tracking-wider border-b app-border pb-2">
                    <Link size={16} />
                    <span>Repository Config</span>
                  </div>

                  <div>
                    <label className="block text-[11px] font-bold app-text-muted uppercase tracking-wider mb-1">
                      SVN URL
                    </label>
                    <input
                      type="text"
                      value={repositoryUrl}
                      onChange={(e) => setRepositoryUrl(e.target.value)}
                      className="w-full app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary placeholder:app-text-muted focus:outline-none font-mono"
                    />
                  </div>

                  <div>
                    <label className="block text-[11px] font-bold app-text-muted uppercase tracking-wider mb-1">
                      Repository Type
                    </label>
                    <select
                      value={repoType}
                      onChange={(e) => setRepoType(e.target.value as 'svn')}
                      className="w-full app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary font-mono"
                    >
                      <option value="svn">svn (Subversion)</option>
                    </select>
                  </div>
                </div>

                {/* Build Strategy & Target Execution */}
                <div className="app-bg-main border app-border rounded-lg p-4 space-y-3">
                  <div className="flex items-center gap-2 text-xs font-bold app-text-primary uppercase tracking-wider border-b app-border pb-2">
                    <Stack size={16} />
                    <span>Build Strategy & Execution</span>
                  </div>

                  <div>
                    <label className="block text-[11px] font-bold app-text-muted uppercase tracking-wider mb-1">
                      Build Strategy (strategy)
                    </label>
                    <select
                      value={buildStrategy}
                      onChange={(e) => setBuildStrategy(e.target.value as 'fail-fast' | 'continue')}
                      className="w-full app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary font-mono"
                    >
                      <option value="fail-fast">fail-fast (Stop on first target failure)</option>
                      <option value="continue">continue (Build remaining targets after failure)</option>
                    </select>
                  </div>
                </div>

                {/* Deployment Options */}
                <div className="app-bg-main border app-border rounded-lg p-4 space-y-3">
                  <div className="flex items-center gap-2 text-xs font-bold app-text-primary uppercase tracking-wider border-b app-border pb-2">
                    <HardDrives size={16} />
                    <span>Deployment & Output Publishing</span>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-[11px] font-bold app-text-muted uppercase tracking-wider mb-1">
                        Publish Policy (publish)
                      </label>
                      <select
                        value={publishPolicy}
                        onChange={(e) => setPublishPolicy(e.target.value as 'all-or-nothing' | 'successful-only')}
                        className="w-full app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary font-mono"
                      >
                        <option value="all-or-nothing">all-or-nothing</option>
                        <option value="successful-only">successful-only</option>
                      </select>
                    </div>

                    <div>
                      <label className="block text-[11px] font-bold app-text-muted uppercase tracking-wider mb-1">
                        Replace Existing
                      </label>
                      <select
                        value={replaceExisting ? 'true' : 'false'}
                        onChange={(e) => setReplaceExisting(e.target.value === 'true')}
                        className="w-full app-bg-input border app-border rounded px-3 py-2 text-xs app-text-primary font-mono"
                      >
                        <option value="true">true (Overwrite folder)</option>
                        <option value="false">false (Skip if exists)</option>
                      </select>
                    </div>
                  </div>
                </div>

                {/* Projects List Configurator */}
                <div className="app-bg-main border app-border rounded-lg p-4 space-y-3">
                  <div className="flex items-center justify-between border-b app-border pb-2">
                    <div className="flex items-center gap-2 text-xs font-bold app-text-primary uppercase tracking-wider">
                      <FileCode size={16} />
                      <span>Projects ({projects.length})</span>
                    </div>
                    <button
                      type="button"
                      onClick={handleAddProject}
                      className="flex items-center gap-1 text-[11px] font-bold app-badge px-2.5 py-1 rounded uppercase tracking-wider cursor-pointer"
                    >
                      <Plus size={14} />
                      <span>Add Project</span>
                    </button>
                  </div>

                  <div className="space-y-4 max-h-72 overflow-y-auto pr-1">
                    {projects.map((p, idx) => (
                      <div key={idx} className="app-bg-card border app-border rounded p-3 space-y-2.5">
                        <div className="flex items-center justify-between">
                          <label className="flex items-center gap-2 text-xs app-text-primary font-bold select-none cursor-pointer">
                            <input
                              type="checkbox"
                              checked={p.isTarget}
                              onChange={(e) => handleUpdateProject(idx, 'isTarget', e.target.checked)}
                              className="w-4 h-4 rounded border-zinc-700 app-bg-main text-current"
                            />
                            <span>Include in Build Targets</span>
                          </label>

                          <button
                            type="button"
                            onClick={() => handleRemoveProject(idx)}
                            className="app-text-muted hover:app-text-primary p-1 cursor-pointer"
                            title="Remove Project"
                          >
                            <Trash size={14} />
                          </button>
                        </div>

                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <span className="block text-[10px] app-text-muted font-bold uppercase">Name</span>
                            <input
                              type="text"
                              value={p.name}
                              onChange={(e) => handleUpdateProject(idx, 'name', e.target.value)}
                              className="w-full app-bg-input border app-border rounded px-2.5 py-1.5 text-xs app-text-primary font-mono"
                            />
                          </div>

                          <div>
                            <span className="block text-[10px] app-text-muted font-bold uppercase">Subfolder Path (path)</span>
                            <input
                              type="text"
                              value={p.path}
                              onChange={(e) => handleUpdateProject(idx, 'path', e.target.value)}
                              className="w-full app-bg-input border app-border rounded px-2.5 py-1.5 text-xs app-text-primary font-mono"
                            />
                          </div>
                        </div>

                        <div>
                          <span className="block text-[10px] app-text-muted font-bold uppercase">Deployment Output Path (output)</span>
                          <input
                            type="text"
                            value={p.output}
                            onChange={(e) => handleUpdateProject(idx, 'output', e.target.value)}
                            className="w-full app-bg-input border app-border rounded px-2.5 py-1.5 text-xs app-text-primary font-mono"
                          />
                        </div>

                        <div>
                          <span className="block text-[10px] app-text-muted font-bold uppercase">
                            Build Script Override (build.script - e.g. "deploy")
                          </span>
                          <input
                            type="text"
                            placeholder="build (default)"
                            value={p.buildScript || ''}
                            onChange={(e) => handleUpdateProject(idx, 'buildScript', e.target.value)}
                            className="w-full app-bg-input border app-border rounded px-2.5 py-1.5 text-xs app-text-primary font-mono placeholder:app-text-muted"
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {/* Live Generated YAML Code Preview Column */}
              <div className="app-bg-main border app-border rounded-lg p-4 flex flex-col h-full">
                <div className="flex items-center justify-between border-b app-border pb-2 mb-3">
                  <span className="text-xs font-bold app-text-primary uppercase tracking-wider font-mono">
                    Live YAML Generated Output
                  </span>
                  <span className="text-[10px] app-text-muted font-mono">cicd.yaml</span>
                </div>
                <textarea
                  readOnly
                  value={generatedYaml}
                  className="w-full flex-1 min-h-[400px] app-bg-card app-text-primary border app-border rounded p-3 font-mono text-[11px] leading-relaxed resize-none focus:outline-none select-all"
                />
              </div>
            </div>
          )}

          {/* MODE 2: RAW YAML EDITOR */}
          {mode === 'raw' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <label className="block text-xs font-bold app-text-primary uppercase tracking-wider font-mono">
                  Edit Raw Pipeline YAML Configuration
                </label>
                <span className="text-[10px] app-text-muted font-mono">yaml content</span>
              </div>
              <textarea
                rows={16}
                value={customRawYaml ?? generatedYaml}
                onChange={(e) => setCustomRawYaml(e.target.value)}
                placeholder="Paste or write custom YAML here..."
                className="w-full app-bg-main border app-border rounded p-4 font-mono text-xs app-text-primary placeholder:app-text-muted focus:outline-none leading-relaxed"
              />
            </div>
          )}

          {/* MODE 3: SIMPLE SVN URL */}
          {mode === 'url' && (
            <div className="space-y-4 max-w-lg mx-auto py-4">
              <div>
                <label className="block text-xs font-bold app-text-secondary uppercase tracking-wider mb-1">
                  SVN Repository URL *
                </label>
                <input
                  type="text"
                  placeholder="https://versionvault.com"
                  value={repositoryUrl}
                  onChange={(e) => setRepositoryUrl(e.target.value)}
                  className="w-full app-bg-main border app-border rounded px-3.5 py-2.5 text-xs app-text-primary placeholder:app-text-muted focus:outline-none font-mono"
                />
              </div>
            </div>
          )}

          {/* Credentials & Revision Section */}
          <div className="pt-4 border-t app-border grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label className="block text-[10px] font-bold app-text-muted uppercase tracking-wider mb-1">
                SVN Username
              </label>
              <input
                type="text"
                placeholder="Username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full app-bg-main border app-border rounded px-3 py-2 text-xs app-text-primary placeholder:app-text-muted focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-[10px] font-bold app-text-muted uppercase tracking-wider mb-1">
                SVN Password
              </label>
              <input
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full app-bg-main border app-border rounded px-3 py-2 text-xs app-text-primary placeholder:app-text-muted focus:outline-none"
              />
            </div>

            <div>
              <label className="block text-[10px] font-bold app-text-muted uppercase tracking-wider mb-1">
                Revision
              </label>
              <input
                type="text"
                placeholder="HEAD"
                value={revision}
                onChange={(e) => setRevision(e.target.value)}
                className="w-full app-bg-main border app-border rounded px-3 py-2 text-xs app-text-primary placeholder:app-text-muted focus:outline-none font-mono"
              />
            </div>
          </div>

          <div className="flex items-center justify-between">
            <label className="flex items-center gap-2 cursor-pointer text-xs app-text-secondary select-none">
              <input
                type="checkbox"
                checked={keepWorkspace}
                onChange={(e) => setKeepWorkspace(e.target.checked)}
                className="w-4 h-4 rounded border-zinc-700 app-bg-main text-current"
              />
              <span>Keep workspace directory on disk after build</span>
            </label>
          </div>
        </form>

        {/* Modal Footer */}
        <div className="px-6 py-4 border-t app-border app-bg-main flex items-center justify-end gap-3 flex-shrink-0">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-xs font-bold app-text-muted hover:app-text-primary uppercase tracking-wider cursor-pointer"
          >
            Cancel
          </button>

          <button
            type="button"
            onClick={handleSubmit}
            disabled={isSubmitting}
            className="flex items-center gap-2 app-badge font-bold text-xs px-6 py-2.5 rounded uppercase tracking-wider transition-all active:scale-95 disabled:opacity-50 shadow-lg cursor-pointer"
          >
            {isSubmitting ? (
              <span>Starting Pipeline...</span>
            ) : (
              <>
                <Play size={14} weight="fill" />
                <span>Run Pipeline</span>
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};
