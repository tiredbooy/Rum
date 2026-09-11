export namespace main {
	
	export class DesktopCapabilities {
	    tray: boolean;
	    folderPicker: boolean;
	    platform: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopCapabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tray = source["tray"];
	        this.folderPicker = source["folderPicker"];
	        this.platform = source["platform"];
	    }
	}

}

