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
	export class TabConfig {
	    id: string;
	    name: string;
	    workDir: string;
	    sessionId: string;
	    soundOverride: string;
	    profileId: string;
	    model: string;
	    type: string;
	
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
	        this.type = source["type"];
	    }
	}
	export class AppConfig {
	    defaultSound: string;
	    theme: string;
	    permissionMode: string;
	    projectName: string;
	    tabs: TabConfig[];
	    profiles: Profile[];
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultSound = source["defaultSound"];
	        this.theme = source["theme"];
	        this.permissionMode = source["permissionMode"];
	        this.projectName = source["projectName"];
	        this.tabs = this.convertValues(source["tabs"], TabConfig);
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

}

