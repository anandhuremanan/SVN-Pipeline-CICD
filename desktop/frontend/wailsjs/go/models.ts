export namespace main {
	
	export class AppInfo {
	    version: string;
	    updateSource: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.updateSource = source["updateSource"];
	    }
	}
	export class ProjectTarget {
	    folder: string;
	    script: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.script = source["script"];
	    }
	}
	export class BuildRequest {
	    repositoryUrl: string;
	    username: string;
	    password: string;
	    outputDir: string;
	    revision: string;
	    shareUsername: string;
	    sharePassword: string;
	    projects: ProjectTarget[];
	    cleanBuild: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BuildRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repositoryUrl = source["repositoryUrl"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.outputDir = source["outputDir"];
	        this.revision = source["revision"];
	        this.shareUsername = source["shareUsername"];
	        this.sharePassword = source["sharePassword"];
	        this.projects = this.convertValues(source["projects"], ProjectTarget);
	        this.cleanBuild = source["cleanBuild"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DriveInfo {
	    root: string;
	    folder: string;
	    freeBytes: number;
	    totalBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new DriveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.folder = source["folder"];
	        this.freeBytes = source["freeBytes"];
	        this.totalBytes = source["totalBytes"];
	    }
	}
	
	export class RemoteFolder {
	    folder: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.type = source["type"];
	    }
	}
	export class Settings {
	    repositoryUrl: string;
	    username: string;
	    outputDir: string;
	    shareUsername: string;
	    projects: ProjectTarget[];
	    storageDir: string;
	    theme: string;
	    recentRepositories: string[];
	    packageKeepDays: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repositoryUrl = source["repositoryUrl"];
	        this.username = source["username"];
	        this.outputDir = source["outputDir"];
	        this.shareUsername = source["shareUsername"];
	        this.projects = this.convertValues(source["projects"], ProjectTarget);
	        this.storageDir = source["storageDir"];
	        this.theme = source["theme"];
	        this.recentRepositories = source["recentRepositories"];
	        this.packageKeepDays = source["packageKeepDays"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StartResult {
	    started: boolean;
	    needsSignIn: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StartResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.started = source["started"];
	        this.needsSignIn = source["needsSignIn"];
	        this.message = source["message"];
	    }
	}
	export class StorageInfo {
	    folder: string;
	    workspaceDir: string;
	    workspaceFreeBytes: number;
	    dataDir: string;
	    dataFreeBytes: number;
	    autoWorkspaceDir: string;
	    autoWorkspaceFreeBytes: number;
	    autoDataDir: string;
	    autoDataFreeBytes: number;
	    drives: DriveInfo[];
	    warning: string;
	    savedPackages: number;
	    packageKeepDays: number;
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.workspaceDir = source["workspaceDir"];
	        this.workspaceFreeBytes = source["workspaceFreeBytes"];
	        this.dataDir = source["dataDir"];
	        this.dataFreeBytes = source["dataFreeBytes"];
	        this.autoWorkspaceDir = source["autoWorkspaceDir"];
	        this.autoWorkspaceFreeBytes = source["autoWorkspaceFreeBytes"];
	        this.autoDataDir = source["autoDataDir"];
	        this.autoDataFreeBytes = source["autoDataFreeBytes"];
	        this.drives = this.convertValues(source["drives"], DriveInfo);
	        this.warning = source["warning"];
	        this.savedPackages = source["savedPackages"];
	        this.packageKeepDays = source["packageKeepDays"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolStatus {
	    key: string;
	    name: string;
	    installed: boolean;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    version: string;
	    notes: string;
	    problem: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.problem = source["problem"];
	    }
	}

}

