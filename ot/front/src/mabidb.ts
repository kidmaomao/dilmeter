import { resVerCall, resDataCall, init as initApi } from '@/lib/apicall';
import { ResourceData, ResourceVersion } from '@/protos/resourcedata';

type OnlyArray<T> = T extends Array<infer V> ? V[] : never;

type ListTypeKeyof<T> = {
    [P in keyof T]-?: T[P] extends OnlyArray<T[P]> ? P : never;
}[keyof T];

export class MabiDB {
    private static dbName = "prilus_mabi_db";
    private static dataTableName = "data";
    private static versionTableName = "version";
    private static updateCheckInterval = 300;

    private dbHandle?: IDBDatabase;

    private cachedRegionString: Record<string, string> = {};
    private cachedLangString: Record<string, string> = {};

    public constructor(private region: string, private lang: string) {
    }

    async open(): Promise<void> {
        this.dbHandle?.close();
        this.dbHandle = undefined;

        // this.isLoading.value = true;

        await new Promise<void>((resolve, reject) => {
            const dbReq = indexedDB.open(MabiDB.dbName, 2);

            dbReq.onupgradeneeded = () => {
                console.log("MabiDB.open: onupgradeneeded");

                const dbHandle = dbReq.result;
                dbHandle.createObjectStore(MabiDB.dataTableName);
                dbHandle.createObjectStore(MabiDB.versionTableName);
            };

            dbReq.onsuccess = () => {
                console.log("MabiDB.open: onsuccess");

                this.dbHandle = dbReq.result;
                resolve();
            }

            dbReq.onerror = () => {
                console.log("MabiDB.open: onerror");

                indexedDB.deleteDatabase(MabiDB.dbName);
                reject(dbReq.error);
            }
        });

        // Resource updates are helpful but must never prevent the desktop app
        // from opening with an already cached CN database while offline.
        try {
            await this.checkUpdate(this.region);
            if (this.region !== this.lang) {
                await this.checkUpdate(this.lang);
            }
        } catch (error) {
            console.warn("MabiDB.open: resource update failed; using cached data", error);
        }

        await this.reloadStringTable();
    }

    public async tryOpen(): Promise<void> {
        if (this.dbHandle) {
            return;
        }

        await initApi();
        await this.open();
    }

    public getData<T extends keyof ResourceData>(key: T, region = this.region): Promise<ResourceData[T]> {
        const rotx = this.dbHandle?.transaction(MabiDB.dataTableName, "readonly");
        if (!rotx) {
            throw new Error("MabiDB.getData: dbHandle is not ready");
        }

        const store = rotx.objectStore(MabiDB.dataTableName);
        const getReq = store.get(`${key}_${region}`);
        return new Promise((resolve, reject) => {
            getReq.onsuccess = () => {
                resolve(getReq.result as ResourceData[T]);
            }

            getReq.onerror = () => {
                reject(getReq.error);
            }
        });
    }

    public getSortedListData<T extends ListTypeKeyof<ResourceData>>(key: T, region = this.region): Promise<ResourceData[T]> {
        const rotx = this.dbHandle?.transaction(MabiDB.dataTableName, "readonly");
        if (!rotx) {
            throw new Error("MabiDB.getData: dbHandle is not ready");
        }

        const store = rotx.objectStore(MabiDB.dataTableName);
        const getReq = store.get(`${key}_${region}`);
        return new Promise((resolve, reject) => {
            getReq.onsuccess = () => {
                const l = getReq.result as ResourceData[T] || [];

                l.sort((a, b) => {
                    const aKey = keyGetter(key, a);
                    const bKey = keyGetter(key, b);

                    return aKey - bKey;
                });

                resolve(l);
            }

            getReq.onerror = () => {
                reject(getReq.error);
            }
        });
    }

    private getVersion(type: string, region: string): Promise<number> {
        const rotx = this.dbHandle?.transaction(MabiDB.versionTableName, "readonly");
        if (!rotx) {
            throw new Error("MabiDB.getVersion: dbHandle is not ready");
        }

        const store = rotx.objectStore(MabiDB.versionTableName);
        const getReq = store.get(`${type}_${region}`);
        return new Promise((resolve, reject) => {
            getReq.onsuccess = () => {
                resolve(+(getReq.result || '0'));
            }

            getReq.onerror = () => {
                reject(getReq.error);
            }
        });
    }

    public async checkUpdate(region: string): Promise<void> {
        const latestVersionCheckAt = await this.getVersion("LatestVersionCheckAt", region) || 0;
        const isTooOldVersionCheck = (Date.now() / 1000) - MabiDB.updateCheckInterval > latestVersionCheckAt;

        console.log("MabiDB.checkUpdate: isTooOldVersionCheck", latestVersionCheckAt, region)

        if (!isTooOldVersionCheck) {
            console.log("MabiDB.checkUpdate: not need update", region);
            return;
        }

        const latestVersion = (await loadServerResourceVersion(region))?.CreatedAt ?? 0;
        const currentVersion = await this.getVersion("CreatedAt", region) ?? 0;

        console.log("MabiDB.checkUpdate: latestVersion", latestVersion, "currentVersion", currentVersion, region)

        if (latestVersion <= currentVersion) {
            this.saveVersion("LatestVersionCheckAt", region, Date.now() / 1000);

            console.log("MabiDB.checkUpdate: not need update", region);
            return;
        }

        await this.update(region);
    }

    public async forceUpdate(): Promise<void> {
        await this.update(this.region);
        if (this.region !== this.lang) {
            await this.update(this.lang);
        }
    }

    private async update(region: string): Promise<void> {
        const { loadingCount } = await import('@/store');
        try {
            loadingCount.value++;

            const data = await loadServerResourceData(region);
            await this.saveDataAll(data, region);

            const version = await loadServerResourceVersion(region);
            await this.saveVersion("CreatedAt", region, data.Version?.CreatedAt || 0);
            await this.saveVersion("LatestVersionCheckAt", region, Date.now() / 1000);

            await this.reloadStringTable();
            console.log("MabiDB.update: updated", version.CreatedAt, region);
        }
        finally {
            loadingCount.value--;
        }
    }

    private clearData(): Promise<void> {
        const dbHandle = this.dbHandle;
        if (!dbHandle) {
            throw new Error("MabiDB.saveDataAll: dbHandle is not ready");
        }

        const tx = dbHandle.transaction(MabiDB.dataTableName, "readwrite");
        const store = tx.objectStore(MabiDB.dataTableName);

        return new Promise((resolve, reject) => {
            const clearReq = store.clear();
            clearReq.onerror = () => {
                console.error("MabiDB.clearData: clearReq.onerror", clearReq.error);
                reject(clearReq.error);
            }

            clearReq.onsuccess = () => {
                tx.commit();
                resolve();
            }
        });
    }

    private async saveDataAll(data: ResourceData, region: string): Promise<void> {
        const dbHandle = this.dbHandle;
        if (!dbHandle) {
            throw new Error("MabiDB.saveDataAll: dbHandle is not ready");
        }

        const tx = dbHandle.transaction(MabiDB.dataTableName, "readwrite");
        const store = tx.objectStore(MabiDB.dataTableName);

        const promises: Promise<void>[] = [];
        const insertPromise = async (req: IDBRequest<IDBValidKey>) => {
            promises.push(new Promise<void>((resolve, reject) => {
                req.onsuccess = () => {
                    resolve();
                }

                req.onerror = () => {
                    console.error("MabiDB.saveDataAll: putReq.onerror", req.error);
                    reject(req.error);
                }
            }));
        }

        for (const _key in data) {
            const key = _key as keyof ResourceData;
            const putReq = store.put(data[key], `${key}_${region}`);
            await insertPromise(putReq);

            /*
            const list = data[key];
            if (key != 'StringTable' && list instanceof Array) {
                for (const v of list) {
                    const elemKey = `${key}_${region}.${keyGetter(key as any, v)}`;
                    const elemPutReq = store.put(v, elemKey);
                    await insertPromise(elemPutReq);
                }
            }
            */
        }

        await Promise.all(promises);

        tx.commit();
    }

    private async saveVersion(type: string, region: string, version: number): Promise<void> {
        const dbHandle = this.dbHandle;
        if (!dbHandle) {
            throw new Error("MabiDB.saveVersion: dbHandle is not ready");
        }

        const tx = dbHandle.transaction(MabiDB.versionTableName, "readwrite");
        const store = tx.objectStore(MabiDB.versionTableName);
        const putReq = store.put(version, `${type}_${region}`);
        await new Promise<void>((resolve, reject) => {
            putReq.onsuccess = () => {
                resolve();
            }

            putReq.onerror = () => {
                console.error("MabiDB.saveVersion: putReq.onerror", putReq.error);
                reject(putReq.error);
            }
        });

        tx.commit();
    }

    public getDataVersion(): Promise<number> {
        return this.getVersion("CreatedAt", this.region);
    }

    private async reloadStringTable(): Promise<void> {
        const region = await this.getData("StringTable") || [];
        const lang = await this.getData("StringTable", this.lang) || [];

        for (const { Id, Str } of region) {
            this.cachedRegionString[Id] = Str;
        }

        for (const { Id, Str } of lang) {
            this.cachedLangString[Id] = Str;
        }
    }

    public getCurLangString(key: string): string {
        if (key == "None") {
            return key;
        }

        const langStr = this.cachedLangString[key];
        if (langStr) {
            return langStr;
        }

        const regionStr = this.cachedRegionString[key];
        if (regionStr) {
            return regionStr;
        }

        return key;
    }

    public getCurLangStrings(keys: string[]): string[] {
        return keys.map(key => this.getCurLangString(key));
    }
}

async function loadServerResourceData(region: string): Promise<ResourceData> {
    const d = await resDataCall(`resourcedata/${region}/${region}_resourcedata.bin.br`);

    return d;
}

async function loadServerResourceVersion(region: string): Promise<ResourceVersion> {
    const d = await resVerCall(`resourceversion/${region}/${region}_resourceversion.json`);

    return d;
}

function keyGetter<T extends ListTypeKeyof<ResourceData>>(typeName: T, d: ResourceData[T][0]): number {
    if (typeName == 'AchievementList') {
        const achievement = d as ResourceData['AchievementList'][0];
        return achievement.Id;
    }
    else if (typeName == 'BarterList') {
        const barter = d as ResourceData['BarterList'][0];
        return barter.Id;
    }
    else if (typeName == 'CharCondList') {
        const charCond = d as ResourceData['CharCondList'][0];
        return charCond.Id;
    }
    else if (typeName == 'FoodList') {
        const food = d as ResourceData['FoodList'][0];
        return food.Id;
    }
    else if (typeName == 'ItemExtendMetalWareList') {
        const item = d as ResourceData['ItemExtendMetalWareList'][0];
        return item.Id;
    }
    else if (typeName == 'ItemExtendUpgradeList') {
        const item = d as ResourceData['ItemExtendUpgradeList'][0];
        return item.Id;
    }
    else if (typeName == 'ItemList') {
        const item = d as ResourceData['ItemList'][0];
        return item.Id;
    }
    else if (typeName == 'ItemUpgradeList') {
        const item = d as ResourceData['ItemUpgradeList'][0];
        return item.Id;
    }
    else if (typeName == 'MetalWareAbilityList') {
        const item = d as ResourceData['ItemExtendMetalWareList'][0];
        return item.Id;
    }
    else if (typeName == 'MetalWareItemList') {
        const item = d as ResourceData['MetalWareItemList'][0];
        return item.Id;
    }
    else if (typeName == 'MetalWareLevelList') {
        const item = d as ResourceData['MetalWareLevelList'][0];
        return item.Level;
    }
    else if (typeName == 'MiniatureList') {
        const item = d as ResourceData['MiniatureList'][0];
        return item.Id;
    }
    else if (typeName == 'OptionSetList') {
        const item = d as ResourceData['OptionSetList'][0];
        return item.Id;
    }
    else if (typeName == 'PetList') {
        const item = d as ResourceData['PetList'][0];
        return item.Id;
    }
    else if (typeName == 'ProductionList') {
        const item = d as ResourceData['ProductionList'][0];
        return item.ItemId;
    }
    else if (typeName == 'RaceList') {
        const item = d as ResourceData['RaceList'][0];
        return item.Id;
    }
    else if (typeName == 'RandomTableList') {
        const item = d as ResourceData['RandomTableList'][0];
        return item.Id;
    }
    else if (typeName == 'SkillList') {
        const item = d as ResourceData['SkillList'][0];
        return item.Id;
    }
    else if (typeName == 'SocialMotionList') {
        const item = d as ResourceData['SocialMotionList'][0];
        return item.Id;
    }
    else if (typeName == 'TalentList') {
        const item = d as ResourceData['TalentList'][0];
        return item.Id;
    }
    else if (typeName == 'TitleList') {
        const item = d as ResourceData['TitleList'][0];
        return item.Id;
    }

    throw new Error(`keyGetter: unknown typeName ${typeName}`);
}
