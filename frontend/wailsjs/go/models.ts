export namespace agents {
	
	export class Capabilities {
	    newSession: boolean;
	    resumeById: boolean;
	    continueLast: boolean;
	    assignSessionId: boolean;
	    initialPrompt: boolean;
	    readsInstructionFiles: boolean;
	    nonInteractive: boolean;
	    permissionControls: boolean;
	    workingDirFlag: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.newSession = source["newSession"];
	        this.resumeById = source["resumeById"];
	        this.continueLast = source["continueLast"];
	        this.assignSessionId = source["assignSessionId"];
	        this.initialPrompt = source["initialPrompt"];
	        this.readsInstructionFiles = source["readsInstructionFiles"];
	        this.nonInteractive = source["nonInteractive"];
	        this.permissionControls = source["permissionControls"];
	        this.workingDirFlag = source["workingDirFlag"];
	    }
	}
	export class ChatSpec {
	    newArgs: string[];
	    resumeArgs: string[];
	    continueArgs: string[];
	    sessionIdArgs: string[];
	    editArgs: string[];
	    promptVia: string;
	    assignSessionId: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChatSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.newArgs = source["newArgs"];
	        this.resumeArgs = source["resumeArgs"];
	        this.continueArgs = source["continueArgs"];
	        this.sessionIdArgs = source["sessionIdArgs"];
	        this.editArgs = source["editArgs"];
	        this.promptVia = source["promptVia"];
	        this.assignSessionId = source["assignSessionId"];
	    }
	}
	export class LaunchSpec {
	    exe: string;
	    args: string[];
	    dir: string;
	    env: string[];
	    notes: string[];
	    handoff: string;
	    externalId: string;
	    display: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exe = source["exe"];
	        this.args = source["args"];
	        this.dir = source["dir"];
	        this.env = source["env"];
	        this.notes = source["notes"];
	        this.handoff = source["handoff"];
	        this.externalId = source["externalId"];
	        this.display = source["display"];
	    }
	}
	export class Profile {
	    id: string;
	    displayName: string;
	    executables: string[];
	    versionArgs: string[];
	    docs: string;
	    os?: string[];
	    capabilities: Capabilities;
	    newArgs: string[];
	    promptArgs: string[];
	    resumeArgs: string[];
	    continueArgs: string[];
	    sessionIdArgs: string[];
	    handoff: string;
	    verified: boolean;
	    verifiedVersion: string;
	    builtin: boolean;
	    notes: string;
	    envNames?: string[];
	    extraArgs?: string[];
	    modelArgs?: string[];
	    modelHints?: string[];
	    chat?: ChatSpec;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.executables = source["executables"];
	        this.versionArgs = source["versionArgs"];
	        this.docs = source["docs"];
	        this.os = source["os"];
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
	        this.newArgs = source["newArgs"];
	        this.promptArgs = source["promptArgs"];
	        this.resumeArgs = source["resumeArgs"];
	        this.continueArgs = source["continueArgs"];
	        this.sessionIdArgs = source["sessionIdArgs"];
	        this.handoff = source["handoff"];
	        this.verified = source["verified"];
	        this.verifiedVersion = source["verifiedVersion"];
	        this.builtin = source["builtin"];
	        this.notes = source["notes"];
	        this.envNames = source["envNames"];
	        this.extraArgs = source["extraArgs"];
	        this.modelArgs = source["modelArgs"];
	        this.modelHints = source["modelHints"];
	        this.chat = this.convertValues(source["chat"], ChatSpec);
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
	export class Status {
	    id: string;
	    state: string;
	    path: string;
	    version: string;
	    detail: string;
	    verified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.detail = source["detail"];
	        this.verified = source["verified"];
	    }
	}

}

export namespace commands {
	
	export class Command {
	    name: string;
	    description: string;
	    usage: string;
	
	    static createFrom(source: any = {}) {
	        return new Command(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.usage = source["usage"];
	    }
	}

}

export namespace contextbuilder {
	
	export class FileRef {
	    path: string;
	    reason: string;
	    tokens: number;
	
	    static createFrom(source: any = {}) {
	        return new FileRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	        this.tokens = source["tokens"];
	    }
	}
	export class ContextPackage {
	    markdown: string;
	    tokens: number;
	    budget: number;
	    included: FileRef[];
	    excluded: FileRef[];
	    version: number;
	    hash: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextPackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.markdown = source["markdown"];
	        this.tokens = source["tokens"];
	        this.budget = source["budget"];
	        this.included = this.convertValues(source["included"], FileRef);
	        this.excluded = this.convertValues(source["excluded"], FileRef);
	        this.version = source["version"];
	        this.hash = source["hash"];
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

}

export namespace history {
	
	export class TestRun {
	    command: string;
	    result: string;
	
	    static createFrom(source: any = {}) {
	        return new TestRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.result = source["result"];
	    }
	}
	export class HandoffInput {
	    task: string;
	    agent: string;
	    sessionId: string;
	    progress: string;
	    filesChanged: string[];
	    tests: TestRun[];
	    issues: string;
	    nextSteps: string;
	
	    static createFrom(source: any = {}) {
	        return new HandoffInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task = source["task"];
	        this.agent = source["agent"];
	        this.sessionId = source["sessionId"];
	        this.progress = source["progress"];
	        this.filesChanged = source["filesChanged"];
	        this.tests = this.convertValues(source["tests"], TestRun);
	        this.issues = source["issues"];
	        this.nextSteps = source["nextSteps"];
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

}

export namespace markdown {
	
	export class AuditEntry {
	    // Go type: time
	    time: any;
	    action: string;
	    path: string;
	    backup?: string;
	    newHash: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.action = source["action"];
	        this.path = source["path"];
	        this.backup = source["backup"];
	        this.newHash = source["newHash"];
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
	export class Preview {
	    path: string;
	    exists: boolean;
	    changed: boolean;
	    diff: string;
	    baseHash: string;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.changed = source["changed"];
	        this.diff = source["diff"];
	        this.baseHash = source["baseHash"];
	    }
	}

}

export namespace scanner {
	
	export class GitInfo {
	    isRepo: boolean;
	    branch: string;
	    dirty: number;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new GitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isRepo = source["isRepo"];
	        this.branch = source["branch"];
	        this.dirty = source["dirty"];
	        this.message = source["message"];
	    }
	}
	export class Item {
	    path: string;
	    category: string;
	    size: number;
	    // Go type: time
	    modTime: any;
	    secret: boolean;
	    injection: boolean;
	    tooLarge: boolean;
	    action: string;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.category = source["category"];
	        this.size = source["size"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.secret = source["secret"];
	        this.injection = source["injection"];
	        this.tooLarge = source["tooLarge"];
	        this.action = source["action"];
	        this.warning = source["warning"];
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
	export class Result {
	    root: string;
	    items: Item[];
	    git: GitInfo;
	    state: string;
	    warnings: string[];
	    truncated: boolean;
	    duplicates: string[];
	    tree: string[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.items = this.convertValues(source["items"], Item);
	        this.git = this.convertValues(source["git"], GitInfo);
	        this.state = source["state"];
	        this.warnings = source["warnings"];
	        this.truncated = source["truncated"];
	        this.duplicates = source["duplicates"];
	        this.tree = source["tree"];
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

}

export namespace service {
	
	export class AgentInfo {
	    profile: agents.Profile;
	    status: agents.Status;
	
	    static createFrom(source: any = {}) {
	        return new AgentInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], agents.Profile);
	        this.status = this.convertValues(source["status"], agents.Status);
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
	export class Settings {
	    defaultAgent: string;
	    defaultMode: string;
	    budget: number;
	    ignoreDirs: string[];
	    ignoreGlobs: string[];
	    maxFileKB: number;
	    contextDir: string;
	    confirmWrites: boolean;
	    theme: string;
	    terminal: string;
	    agentPaths: Record<string, string>;
	    agentModels: Record<string, string>;
	    autoTerminal: boolean;
	    chatEdits: boolean;
	    voiceCommand: string;
	    voiceModel: string;
	    voiceLanguage: string;
	    pinned: string[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultAgent = source["defaultAgent"];
	        this.defaultMode = source["defaultMode"];
	        this.budget = source["budget"];
	        this.ignoreDirs = source["ignoreDirs"];
	        this.ignoreGlobs = source["ignoreGlobs"];
	        this.maxFileKB = source["maxFileKB"];
	        this.contextDir = source["contextDir"];
	        this.confirmWrites = source["confirmWrites"];
	        this.theme = source["theme"];
	        this.terminal = source["terminal"];
	        this.agentPaths = source["agentPaths"];
	        this.agentModels = source["agentModels"];
	        this.autoTerminal = source["autoTerminal"];
	        this.chatEdits = source["chatEdits"];
	        this.voiceCommand = source["voiceCommand"];
	        this.voiceModel = source["voiceModel"];
	        this.voiceLanguage = source["voiceLanguage"];
	        this.pinned = source["pinned"];
	    }
	}
	export class BootInfo {
	    version: string;
	    dataDir: string;
	    initial?: workspace.OpenResult;
	    initialError: string;
	    settings: Settings;
	
	    static createFrom(source: any = {}) {
	        return new BootInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.dataDir = source["dataDir"];
	        this.initial = this.convertValues(source["initial"], workspace.OpenResult);
	        this.initialError = source["initialError"];
	        this.settings = this.convertValues(source["settings"], Settings);
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
	export class ChatState {
	    running: boolean;
	    queued: number;
	
	    static createFrom(source: any = {}) {
	        return new ChatState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.queued = source["queued"];
	    }
	}
	export class ContextRequest {
	    workspaceId: number;
	    budget: number;
	    task: string;
	    knownIssues: string;
	    nextSteps: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.budget = source["budget"];
	        this.task = source["task"];
	        this.knownIssues = source["knownIssues"];
	        this.nextSteps = source["nextSteps"];
	    }
	}
	export class FileEntry {
	    name: string;
	    rel: string;
	    dir: boolean;
	    size: number;
	    secret: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.rel = source["rel"];
	        this.dir = source["dir"];
	        this.size = source["size"];
	        this.secret = source["secret"];
	    }
	}
	export class ImportResult {
	    added: storage.Media[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = this.convertValues(source["added"], storage.Media);
	        this.errors = source["errors"];
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
	export class InstructionStatus {
	    file: string;
	    exists: boolean;
	    hasBlock: boolean;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstructionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.exists = source["exists"];
	        this.hasBlock = source["hasBlock"];
	        this.current = source["current"];
	    }
	}
	export class LaunchOptions {
	    workspaceId: number;
	    agent: string;
	    mode: string;
	    sessionId: number;
	    title: string;
	    task: string;
	    knownIssues: string;
	    nextSteps: string;
	    dryRun: boolean;
	    force: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LaunchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.agent = source["agent"];
	        this.mode = source["mode"];
	        this.sessionId = source["sessionId"];
	        this.title = source["title"];
	        this.task = source["task"];
	        this.knownIssues = source["knownIssues"];
	        this.nextSteps = source["nextSteps"];
	        this.dryRun = source["dryRun"];
	        this.force = source["force"];
	    }
	}
	export class LaunchResult {
	    launched: boolean;
	    spec?: agents.LaunchSpec;
	    sessionId: number;
	    contextFile: string;
	    tokens: number;
	    disclosure: string;
	    error: string;
	    fallback: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.launched = source["launched"];
	        this.spec = this.convertValues(source["spec"], agents.LaunchSpec);
	        this.sessionId = source["sessionId"];
	        this.contextFile = source["contextFile"];
	        this.tokens = source["tokens"];
	        this.disclosure = source["disclosure"];
	        this.error = source["error"];
	        this.fallback = source["fallback"];
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
	export class Lifecycle {
	
	
	    static createFrom(source: any = {}) {
	        return new Lifecycle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class MovePair {
	    from: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new MovePair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class SendOptions {
	    conversationId: number;
	    text: string;
	    mediaIds: number[];
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new SendOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversationId = source["conversationId"];
	        this.text = source["text"];
	        this.mediaIds = source["mediaIds"];
	        this.mode = source["mode"];
	    }
	}
	export class SendResult {
	    message?: storage.Message;
	    sent: boolean;
	    running: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new SendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = this.convertValues(source["message"], storage.Message);
	        this.sent = source["sent"];
	        this.running = source["running"];
	        this.note = source["note"];
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
	
	export class SkillPreview {
	    relPath: string;
	    content: string;
	    preview?: markdown.Preview;
	
	    static createFrom(source: any = {}) {
	        return new SkillPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relPath = source["relPath"];
	        this.content = source["content"];
	        this.preview = this.convertValues(source["preview"], markdown.Preview);
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
	export class SkillsOverview {
	    inventory?: skills.Inventory;
	    templates: skills.Template[];
	    compat: skills.Compat[];
	    recommendations: skills.Recommendation[];
	
	    static createFrom(source: any = {}) {
	        return new SkillsOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inventory = this.convertValues(source["inventory"], skills.Inventory);
	        this.templates = this.convertValues(source["templates"], skills.Template);
	        this.compat = this.convertValues(source["compat"], skills.Compat);
	        this.recommendations = this.convertValues(source["recommendations"], skills.Recommendation);
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
	export class TermInfo {
	    id: string;
	    conversationId: number;
	    agent: string;
	    running: boolean;
	    command: string;
	    notes: string[];
	    sessionId: number;
	    error: string;
	    fallback: string;
	
	    static createFrom(source: any = {}) {
	        return new TermInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.agent = source["agent"];
	        this.running = source["running"];
	        this.command = source["command"];
	        this.notes = source["notes"];
	        this.sessionId = source["sessionId"];
	        this.error = source["error"];
	        this.fallback = source["fallback"];
	    }
	}
	export class TermStartOptions {
	    workspaceId: number;
	    conversationId: number;
	    agent: string;
	    mode: string;
	    cols: number;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new TermStartOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspaceId = source["workspaceId"];
	        this.conversationId = source["conversationId"];
	        this.agent = source["agent"];
	        this.mode = source["mode"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	    }
	}
	export class VoiceResult {
	    media?: storage.Media;
	    transcript: string;
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new VoiceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.media = this.convertValues(source["media"], storage.Media);
	        this.transcript = source["transcript"];
	        this.warning = source["warning"];
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
	export class VoiceStatus {
	    recorder: string;
	    engine: string;
	    ready: boolean;
	    recording: boolean;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new VoiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recorder = source["recorder"];
	        this.engine = source["engine"];
	        this.ready = source["ready"];
	        this.recording = source["recording"];
	        this.hint = source["hint"];
	    }
	}
	export class WorkspaceState {
	    task: string;
	    knownIssues: string;
	    nextSteps: string;
	    excluded: string[];
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task = source["task"];
	        this.knownIssues = source["knownIssues"];
	        this.nextSteps = source["nextSteps"];
	        this.excluded = source["excluded"];
	        this.mode = source["mode"];
	    }
	}
	export class WorkspaceSummary {
	    workspace: storage.Workspace;
	    sessions: number;
	    resumable: number;
	    pinned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], storage.Workspace);
	        this.sessions = source["sessions"];
	        this.resumable = source["resumable"];
	        this.pinned = source["pinned"];
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

}

export namespace skills {
	
	export class Compat {
	    agent: string;
	    dir: string;
	    verified: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new Compat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent = source["agent"];
	        this.dir = source["dir"];
	        this.verified = source["verified"];
	        this.note = source["note"];
	    }
	}
	export class ImportFile {
	    rel: string;
	    size: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rel = source["rel"];
	        this.size = source["size"];
	        this.warnings = source["warnings"];
	    }
	}
	export class ImportPreview {
	    source: string;
	    name: string;
	    files: ImportFile[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.name = source["name"];
	        this.files = this.convertValues(source["files"], ImportFile);
	        this.warnings = source["warnings"];
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
	export class Skill {
	    name: string;
	    description: string;
	    path: string;
	    dir: string;
	    relPath?: string;
	    source: string;
	    agent: string;
	    managed: boolean;
	    templateVersion: number;
	    hash: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.path = source["path"];
	        this.dir = source["dir"];
	        this.relPath = source["relPath"];
	        this.source = source["source"];
	        this.agent = source["agent"];
	        this.managed = source["managed"];
	        this.templateVersion = source["templateVersion"];
	        this.hash = source["hash"];
	        this.warnings = source["warnings"];
	    }
	}
	export class Inventory {
	    skills: Skill[];
	    duplicates: string[];
	    conflicts: string[];
	
	    static createFrom(source: any = {}) {
	        return new Inventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skills = this.convertValues(source["skills"], Skill);
	        this.duplicates = source["duplicates"];
	        this.conflicts = source["conflicts"];
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
	export class Recommendation {
	    template: string;
	    state: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Recommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.template = source["template"];
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}
	
	export class Template {
	    id: string;
	    name: string;
	    description: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.body = source["body"];
	    }
	}

}

export namespace storage {
	
	export class Conversation {
	    id: number;
	    workspaceId: number;
	    title: string;
	    agent: string;
	    sessionId: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    messages: number;
	
	    static createFrom(source: any = {}) {
	        return new Conversation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.title = source["title"];
	        this.agent = source["agent"];
	        this.sessionId = source["sessionId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.messages = source["messages"];
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
	export class Media {
	    id: number;
	    workspaceId: number;
	    name: string;
	    stored: string;
	    kind: string;
	    mime: string;
	    size: number;
	    note: string;
	    inContext: boolean;
	    // Go type: time
	    addedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Media(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.stored = source["stored"];
	        this.kind = source["kind"];
	        this.mime = source["mime"];
	        this.size = source["size"];
	        this.note = source["note"];
	        this.inContext = source["inContext"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
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
	export class Message {
	    id: number;
	    conversationId: number;
	    role: string;
	    source: string;
	    body: string;
	    // Go type: time
	    createdAt: any;
	    media: Media[];
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.role = source["role"];
	        this.source = source["source"];
	        this.body = source["body"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.media = this.convertValues(source["media"], Media);
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
	export class Note {
	    name: string;
	    body: string;
	    // Go type: time
	    updatedAt: any;
	    updatedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.body = source["body"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.updatedBy = source["updatedBy"];
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
	export class NoteVersion {
	    id: number;
	    body: string;
	    // Go type: time
	    savedAt: any;
	    savedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new NoteVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.body = source["body"];
	        this.savedAt = this.convertValues(source["savedAt"], null);
	        this.savedBy = source["savedBy"];
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
	export class Page {
	    id: number;
	    workspaceId: number;
	    title: string;
	    summary: string;
	    body: string;
	    sort: number;
	    // Go type: time
	    updatedAt: any;
	    updatedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.body = source["body"];
	        this.sort = source["sort"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.updatedBy = source["updatedBy"];
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
	export class PageVersion {
	    id: number;
	    title: string;
	    summary: string;
	    body: string;
	    // Go type: time
	    savedAt: any;
	    savedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new PageVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.body = source["body"];
	        this.savedAt = this.convertValues(source["savedAt"], null);
	        this.savedBy = source["savedBy"];
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
	export class Session {
	    id: number;
	    workspaceId: number;
	    agent: string;
	    externalId: string;
	    status: string;
	    title: string;
	    handoff: string;
	    contextVersion: string;
	    exitStatus: string;
	    pid: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastActive: any;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.agent = source["agent"];
	        this.externalId = source["externalId"];
	        this.status = source["status"];
	        this.title = source["title"];
	        this.handoff = source["handoff"];
	        this.contextVersion = source["contextVersion"];
	        this.exitStatus = source["exitStatus"];
	        this.pid = source["pid"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastActive = this.convertValues(source["lastActive"], null);
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
	export class Skill {
	    id: number;
	    workspaceId: number;
	    name: string;
	    description: string;
	    body: string;
	    alwaysOn: boolean;
	    // Go type: time
	    updatedAt: any;
	    updatedBy: string;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.workspaceId = source["workspaceId"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.body = source["body"];
	        this.alwaysOn = source["alwaysOn"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.updatedBy = source["updatedBy"];
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
	export class Workspace {
	    id: number;
	    path: string;
	    name: string;
	    agent: string;
	    settings: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastOpened: any;
	
	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.agent = source["agent"];
	        this.settings = source["settings"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastOpened = this.convertValues(source["lastOpened"], null);
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

}

export namespace workspace {
	
	export class OpenResult {
	    workspace?: storage.Workspace;
	    sessions: storage.Session[];
	    resumable: storage.Session[];
	    action: string;
	    isNew: boolean;
	    requestedNew: boolean;
	    page: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], storage.Workspace);
	        this.sessions = this.convertValues(source["sessions"], storage.Session);
	        this.resumable = this.convertValues(source["resumable"], storage.Session);
	        this.action = source["action"];
	        this.isNew = source["isNew"];
	        this.requestedNew = source["requestedNew"];
	        this.page = source["page"];
	        this.reason = source["reason"];
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

}

