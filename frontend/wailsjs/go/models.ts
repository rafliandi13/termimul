export namespace main {
	
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
	export class KnownHost {
	    id: string;
	    hosts: string;
	    keyType: string;
	    fingerprint: string;
	    hashed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KnownHost(source);
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

