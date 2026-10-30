export namespace knownhosts {
	
	export class Entry {
	    id: string;
	    hosts: string;
	    keyType: string;
	    fingerprint: string;
	    hashed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.hosts = source["hosts"];
	        this.keyType = source["keyType"];
	        this.fingerprint = source["fingerprint"];
	        this.hashed = source["hashed"];
	    }
	}

}

export namespace services {
	
	export class ImportResult {
	    hostsAdded: number;
	    hostsSkipped: number;
	    keysAdded: number;
	    keysSkipped: number;
	    tunnelsAdded: number;
	    snippetsAdded: number;
	    knownHostsAdded: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostsAdded = source["hostsAdded"];
	        this.hostsSkipped = source["hostsSkipped"];
	        this.keysAdded = source["keysAdded"];
	        this.keysSkipped = source["keysSkipped"];
	        this.tunnelsAdded = source["tunnelsAdded"];
	        this.snippetsAdded = source["snippetsAdded"];
	        this.knownHostsAdded = source["knownHostsAdded"];
	    }
	}
	export class RunRequest {
	    command: string;
	    hostIds: string[];
	    concurrency: number;
	    timeoutSec: number;
	
	    static createFrom(source: any = {}) {
	        return new RunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.hostIds = source["hostIds"];
	        this.concurrency = source["concurrency"];
	        this.timeoutSec = source["timeoutSec"];
	    }
	}
	export class SSHConfigHost {
	    alias: string;
	    address: string;
	    port: number;
	    username: string;
	    keyPath: string;
	    keyStatus: string;
	    keyError: string;
	    duplicate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SSHConfigHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.keyPath = source["keyPath"];
	        this.keyStatus = source["keyStatus"];
	        this.keyError = source["keyError"];
	        this.duplicate = source["duplicate"];
	    }
	}
	export class SSHConfigImport {
	    aliases: string[];
	    passphrases: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new SSHConfigImport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.aliases = source["aliases"];
	        this.passphrases = source["passphrases"];
	    }
	}
	export class SSHConfigPreview {
	    path: string;
	    hosts: SSHConfigHost[];
	
	    static createFrom(source: any = {}) {
	        return new SSHConfigPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.hosts = this.convertValues(source["hosts"], SSHConfigHost);
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
	export class SSHConfigResult {
	    hostsAdded: number;
	    hostsSkipped: number;
	    keysAdded: number;
	    noKey: string[];
	
	    static createFrom(source: any = {}) {
	        return new SSHConfigResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostsAdded = source["hostsAdded"];
	        this.hostsSkipped = source["hostsSkipped"];
	        this.keysAdded = source["keysAdded"];
	        this.noKey = source["noKey"];
	    }
	}
	export class TunnelView {
	    id: string;
	    label: string;
	    hostId: string;
	    type: string;
	    bindAddress: string;
	    bindPort: number;
	    targetHost: string;
	    targetPort: number;
	    autoStart: boolean;
	    status: tunnel.Status;
	
	    static createFrom(source: any = {}) {
	        return new TunnelView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.hostId = source["hostId"];
	        this.type = source["type"];
	        this.bindAddress = source["bindAddress"];
	        this.bindPort = source["bindPort"];
	        this.targetHost = source["targetHost"];
	        this.targetPort = source["targetPort"];
	        this.autoStart = source["autoStart"];
	        this.status = this.convertValues(source["status"], tunnel.Status);
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

export namespace sftpx {
	
	export class FileEntry {
	    name: string;
	    path: string;
	    size: number;
	    mode: string;
	    modTime: number;
	    isDir: boolean;
	    isLink: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	        this.modTime = source["modTime"];
	        this.isDir = source["isDir"];
	        this.isLink = source["isLink"];
	    }
	}
	export class Listing {
	    path: string;
	    parent: string;
	    entries: FileEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Listing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.parent = source["parent"];
	        this.entries = this.convertValues(source["entries"], FileEntry);
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

export namespace tunnel {
	
	export class Status {
	    id: string;
	    state: string;
	    error: string;
	    addr: string;
	    connections: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.state = source["state"];
	        this.error = source["error"];
	        this.addr = source["addr"];
	        this.connections = source["connections"];
	    }
	}

}

export namespace vault {
	
	export class Host {
	    id: string;
	    label: string;
	    group: string;
	    address: string;
	    port: number;
	    username: string;
	    authType: string;
	    keyId: string;
	    keyPath: string;
	    hasPassword: boolean;
	    hasPassphrase: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Host(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.group = source["group"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyId = source["keyId"];
	        this.keyPath = source["keyPath"];
	        this.hasPassword = source["hasPassword"];
	        this.hasPassphrase = source["hasPassphrase"];
	    }
	}
	export class HostInput {
	    id: string;
	    label: string;
	    group: string;
	    address: string;
	    port: number;
	    username: string;
	    authType: string;
	    keyId: string;
	    keyPath: string;
	    hasPassword: boolean;
	    hasPassphrase: boolean;
	    password: string;
	    passphrase: string;
	
	    static createFrom(source: any = {}) {
	        return new HostInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.group = source["group"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.keyId = source["keyId"];
	        this.keyPath = source["keyPath"];
	        this.hasPassword = source["hasPassword"];
	        this.hasPassphrase = source["hasPassphrase"];
	        this.password = source["password"];
	        this.passphrase = source["passphrase"];
	    }
	}
	export class Key {
	    id: string;
	    label: string;
	    type: string;
	    fingerprint: string;
	    publicKey: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Key(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.fingerprint = source["fingerprint"];
	        this.publicKey = source["publicKey"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Snippet {
	    id: string;
	    name: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new Snippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	    }
	}
	export class Tunnel {
	    id: string;
	    label: string;
	    hostId: string;
	    type: string;
	    bindAddress: string;
	    bindPort: number;
	    targetHost: string;
	    targetPort: number;
	    autoStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tunnel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.hostId = source["hostId"];
	        this.type = source["type"];
	        this.bindAddress = source["bindAddress"];
	        this.bindPort = source["bindPort"];
	        this.targetHost = source["targetHost"];
	        this.targetPort = source["targetPort"];
	        this.autoStart = source["autoStart"];
	    }
	}

}

