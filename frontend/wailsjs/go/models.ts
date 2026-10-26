export namespace main {
	
	export class Host {
	    id: string;
	    label: string;
	    group: string;
	    address: string;
	    port: number;
	    username: string;
	    authType: string;
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
	        this.keyPath = source["keyPath"];
	        this.hasPassword = source["hasPassword"];
	        this.hasPassphrase = source["hasPassphrase"];
	        this.password = source["password"];
	        this.passphrase = source["passphrase"];
	    }
	}

}

