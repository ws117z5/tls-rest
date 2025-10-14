import { Component } from "react";

import modulesConfig from '../../modules.json'

type Submodule = {
    href: string;
    component: Component;
    props?: Record<string, any>;
}

type Module = {
    href: string;
    name: string;
    title: string;
    uuid: string;
    isPage?: boolean;
    component: Component;
    props?: Record<string, any>;
    extraRoutes: Submodule[];
}



export default class Config {
    private static modules: Module[] = [];
    private static moduleMap = new Map<string, number>();
    public static len: number = 0;
    //public static serverURL = "https://localhost/"
    public static serverURL = window.location.protocol + "//" + window.location.hostname + "/"

    static async init() {

        await import("../components/pages").then((loaded) => {
            //const i = 1;
            //debugger;

            Object.keys(loaded).map((key, index) => {

                if('guid' in loaded[key]) {

                    let _module: Module = {
                        href: loaded[key].href,
                        name: key,
                        uuid: loaded[key].uuid,
                        title: loaded[key].title,
                        isPage: loaded[key].isPage,
                        component: loaded[key],
                        props: loaded[key].props || {},
                        extraRoutes: loaded[key].extraRoutes ? loaded[key].extraRoutes : []
                    };

                    let idx: number = Config.modules.push(_module)
                    Config.moduleMap.set(key, idx-1);
                    Config.len++;

                }
            });
          });
          
    }


    public static get(name: string): Module | null {
        let idx = Config.moduleMap.get(name);
        return idx ? Config.modules[idx] : null;
    }

    public static getAll(): Module[] | null {
        return Config.modules;
    }

    public static getPages(): Module[] | null {
        return Config.modules.filter(module => { return module.isPage == true; });
    }

}
