import { ref, shallowRef, computed } from "vue";
import { MabiDB } from "@/mabidb";
import { ActorManager } from "@/eventActor";
import { DamageCollectorManager } from "@/actionCollector";

export const loadingCount = ref(0);
export const isLoading = computed(() => loadingCount.value > 0);

// support: ["kr",  "cn", "jp", "tw", "us"]
const defaultRegion = "cn";

export const region = ref(defaultRegion);
export const lang = ref(defaultRegion);
export const regionList = ref([defaultRegion]);

export const db = computed(() => {
    const instance = new MabiDB(region.value, lang.value);

    // // fire and forget
    // instance.open().catch(e => console.error(e));

    return instance;
});

// export const raceNameMap = ref<Record<number, string>>({
//     434: "木頭人",
//     4856: "和浣熊一樣虛弱的木頭人",
//     4857: "和泰赫圖殷之門一樣堅固的木頭人",
//     4858: "和魅魔女王一樣堅固的木頭人",
//     4859: "和伊魯夏一樣堅固的木頭人",
//     4860: "和卡莉亞赫一樣堅固的木頭人",
//     7600: "佩塔克1階",
//     7601: "佩塔克2階",
//     7602: "笨馬",
//     7603: "火山孝子還我錢",
//     7604: "安樂的碎片1",
//     7605: "安樂的碎片2",
//     7606: "安樂的碎片3",
//     7607: "安樂的碎片4",
//     7608: "安樂的碎片5",
//     7609: "隆起的地板",
//     7610: "古代意志",
//     7611: "古代巨人1",
//     7612: "古代巨人2",
//     7613: "古代巨人3",
//     7614: "古代巨人4",
// });
// export const skillNameMap = ref<Record<number, string>>(
//     Object.fromEntries(
//         skillData.map((skill) => [
//             skill.SkillID,
//             skill.SkillLocalName ?? skill.SkillEngName,
//         ]),
//     ),
// );
// ── 玩家改名（全域） ──────────────────────────────────────────
export const idMappings = ref<Record<string, string>>({});
export const getDisplayName = (name: string): string => idMappings.value[name] || name;

export const raceNameMap = ref<Record<number, string>>({});
export const skillNameMap = ref<Record<number, string>>({});
export const condNameMap = ref<Record<number, string>>({});
export const itemNameMap = ref<Record<number, string>>({});
export const resourceNameVersion = ref(0);
export const appEvent = ref(new EventTarget());

export const dcManager = shallowRef(new DamageCollectorManager());
export const actorManager = shallowRef(
    new ActorManager(dcManager.value as DamageCollectorManager),
);

export const timeRangeMin = ref<number | null>(null);
export const timeRangeMax = ref<number | null>(null);
export const hasTimeRange = computed(
    () => timeRangeMin.value !== null && timeRangeMax.value !== null,
);

export function setTimeRange(min: number, max: number) {
    timeRangeMin.value = min;
    timeRangeMax.value = max;
}

export function clearTimeRange() {
    timeRangeMin.value = null;
    timeRangeMax.value = null;
}

// config
const CONFIG_STORAGE_KEY = "config";

export interface AppConfig {
    hiddenCCIds: number[];
    hiddenRaceIds: number[];
}

const defaultConfig: AppConfig = {
    hiddenCCIds: [],
    hiddenRaceIds: [],
};

function loadConfig(): AppConfig {
    try {
        const raw = localStorage.getItem(CONFIG_STORAGE_KEY);
        if (raw) return { ...defaultConfig, ...JSON.parse(raw) };
    } catch {
        /* ignore */
    }
    return { ...defaultConfig };
}

function saveConfig() {
    const data: AppConfig = {
        hiddenCCIds: [...hiddenCCIds.value],
        hiddenRaceIds: [...hiddenRaceIds.value],
    };
    localStorage.setItem(CONFIG_STORAGE_KEY, JSON.stringify(data));
}

const _config = loadConfig();

export const hiddenCCIds = ref<Set<number>>(new Set(_config.hiddenCCIds));

export function addHiddenCC(ccId: number) {
    hiddenCCIds.value = new Set(hiddenCCIds.value).add(ccId);
    saveConfig();
}

export function removeHiddenCC(ccId: number) {
    const next = new Set(hiddenCCIds.value);
    next.delete(ccId);
    hiddenCCIds.value = next;
    saveConfig();
}

export const hiddenRaceIds = ref<Set<number>>(new Set(_config.hiddenRaceIds));

export function addHiddenRace(raceId: number) {
    hiddenRaceIds.value = new Set(hiddenRaceIds.value).add(raceId);
    saveConfig();
}

export function removeHiddenRace(raceId: number) {
    const next = new Set(hiddenRaceIds.value);
    next.delete(raceId);
    hiddenRaceIds.value = next;
    saveConfig();
}

function getAutoRegion() {
    try {
        // 1. get IANA Timezone ID (ex "Asia/Taipei", "Asia/Seoul")
        const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

        const lang = navigator.language.toLowerCase();

        if (timeZone.includes("Taipei")) return "tw";
        if (
            timeZone.includes("Shanghai") ||
            timeZone.includes("Chongqing") ||
            timeZone.includes("Urumqi")
        )
            return "cn";
        // 處理香港/澳門 (通常遊戲分區會歸類在 TW 或 CN，看你需求，這裡範例歸在 TW)
        if (timeZone.includes("Hong_Kong")) return "tw";

        // 處理 KR vs JP (UTC+9 衝突)
        if (timeZone.includes("Seoul")) return "kr";
        if (timeZone.includes("Tokyo")) return "jp";

        // 處理 US (美洲時區眾多，用前綴判斷)
        if (timeZone.startsWith("America/")) return "us";

        // --- 模糊比對 (Fallback) ---
        // 如果上面的具體城市都沒抓到 (例如使用者時區設為 Generic Asia/UTC+8)
        // 則改用「語系」來猜

        if (lang.includes("zh-tw") || lang.includes("zh-hk")) return "tw";
        if (lang.includes("zh")) return "cn";
        if (lang.includes("ko")) return "kr";
        if (lang.includes("ja")) return "jp";

        // 預設回傳 (例如以上皆非，預設使用繁體中文台服)
        return "tw";
    } catch (e) {
        console.error("自動偵測失敗:", e);
        return "tw"; // 發生錯誤時的保底
    }
}
