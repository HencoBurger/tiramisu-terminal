export namespace main {
	
	export class Profile {
	    id: string;
	    name: string;
	    homeDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.homeDir = source["homeDir"];
	    }
	}
	export class EffectiveConfig {
	    theme: string;
	    defaultSound: string;
	    permissionMode: string;
	    profiles: Profile[];
	
	    static createFrom(source: any = {}) {
	        return new EffectiveConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.defaultSound = source["defaultSound"];
	        this.permissionMode = source["permissionMode"];
	        this.profiles = this.convertValues(source["profiles"], Profile);
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
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	    }
	}
	export class GlobalConfig {
	    defaultSound: string;
	    theme: string;
	    permissionMode: string;
	    profiles: Profile[];
	    ollamaBaseURL: string;
	    enabledProviders: string[];
	    defaultModels: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new GlobalConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultSound = source["defaultSound"];
	        this.theme = source["theme"];
	        this.permissionMode = source["permissionMode"];
	        this.profiles = this.convertValues(source["profiles"], Profile);
	        this.ollamaBaseURL = source["ollamaBaseURL"];
	        this.enabledProviders = source["enabledProviders"];
	        this.defaultModels = source["defaultModels"];
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
	export class HistoryTool {
	    id: string;
	    name: string;
	    input: string;
	    output: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryTool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.input = source["input"];
	        this.output = source["output"];
	    }
	}
	export class HistoryMessage {
	    role: string;
	    content: string;
	    tools: HistoryTool[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.tools = this.convertValues(source["tools"], HistoryTool);
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
	
	export class ModelInfo {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	
	export class StoredSession {
	    sessionId: string;
	    projectDir: string;
	    firstPrompt: string;
	    messageCount: number;
	    lastModified: number;
	
	    static createFrom(source: any = {}) {
	        return new StoredSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.projectDir = source["projectDir"];
	        this.firstPrompt = source["firstPrompt"];
	        this.messageCount = source["messageCount"];
	        this.lastModified = source["lastModified"];
	    }
	}
	export class TabConfig {
	    id: string;
	    name: string;
	    workDir: string;
	    sessionId: string;
	    soundOverride: string;
	    profileId: string;
	    model: string;
	    provider?: string;
	    type: string;
	    openFiles?: string[];
	    activeFile?: string;
	
	    static createFrom(source: any = {}) {
	        return new TabConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.workDir = source["workDir"];
	        this.sessionId = source["sessionId"];
	        this.soundOverride = source["soundOverride"];
	        this.profileId = source["profileId"];
	        this.model = source["model"];
	        this.provider = source["provider"];
	        this.type = source["type"];
	        this.openFiles = source["openFiles"];
	        this.activeFile = source["activeFile"];
	    }
	}
	export class WindowSession {
	    id: string;
	    name: string;
	    tabs: TabConfig[];
	    defaultWorkDir?: string;
	    themeOverride?: string;
	    soundOverride?: string;
	    permModeOverride?: string;
	    createdAt: number;
	    lastOpenedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tabs = this.convertValues(source["tabs"], TabConfig);
	        this.defaultWorkDir = source["defaultWorkDir"];
	        this.themeOverride = source["themeOverride"];
	        this.soundOverride = source["soundOverride"];
	        this.permModeOverride = source["permModeOverride"];
	        this.createdAt = source["createdAt"];
	        this.lastOpenedAt = source["lastOpenedAt"];
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
	export class WindowSessionSummary {
	    id: string;
	    name: string;
	    tabCount: number;
	    lastOpenedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowSessionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tabCount = source["tabCount"];
	        this.lastOpenedAt = source["lastOpenedAt"];
	    }
	}

}

