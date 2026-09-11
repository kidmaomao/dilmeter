import { customRef } from "vue";
import type { Ref } from "vue";

import { ActorManager, BaseActor, GroupActor } from "@/eventActor";
type Locale = "kr" | "krt" | "cn" | "jp" | "tw" | "us";
const localeConfigs: Record<string, { divisor: number; unit: string }[]> = {
    // 萬進制：每 10^4 進位
    tw: [
        { divisor: 1_0000_0000_0000, unit: "兆" },
        { divisor: 1_0000_0000, unit: "億" },
        { divisor: 1_0000, unit: "萬" },
    ],
    cn: [
        { divisor: 1_0000_0000_0000, unit: "兆" },
        { divisor: 1_0000_0000, unit: "亿" },
        { divisor: 1_0000, unit: "万" },
    ],
    kr: [
        { divisor: 1_0000_0000_0000, unit: "조" },
        { divisor: 1_0000_0000, unit: "억" },
        { divisor: 1_0000, unit: "만" },
    ],
    // 千進制：每 10^3 進位
    us: [
        { divisor: 1_0000_0000_0000, unit: "T" },
        { divisor: 1_000_000_000, unit: "B" },
        { divisor: 1_000_000, unit: "M" },
        { divisor: 1_000, unit: "K" },
    ],
};

export function humanReadableNumber(n: any, locale: Locale = "tw"): string {
    const num = typeof n === "string" ? parseFloat(n) : n;

    if (isNaN(num) || !isFinite(num) || typeof num !== "number") {
        return "0";
    }

    const abs = Math.abs(num);
    const sign = num < 0 ? "-" : "";

    // 取得該語系的配置，若無則回退到 us 或 tw
    const configs = localeConfigs[locale] || localeConfigs["us"];

    for (const config of configs) {
        if (abs >= config.divisor) {
            // 使用 Number() 或 + 符號移除 toFixed 產生的多餘末尾 0
            const formatted = Number((abs / config.divisor).toFixed(2));
            return `${sign}${formatted}${config.unit}`;
        }
    }

    return sign + Math.round(abs).toString();
}

export function formatDuration(seconds: number): string {
    const min = Math.floor(seconds / 60);
    const sec = Math.floor(seconds % 60);
    return `${String(min).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
}

export function getMabiNameColor(name: string): string {
    if (!name?.length) {
        return "#808080";
    }

    // https://mabinoger.com/color_sim.htm
    const colCalc = (i: number) => ((i * 101) % 97) + 159;

    // if (name.length < 3) {
    //     return '#808080';
    // }

    const ccolor = [0, 0, 0];
    // R = (ASCII Char 1,4,7,10
    // G = (ASCII Char 2,5,8,11
    // B = (ASCII Char 3,6,9,12
    //                          * 101) mod 97) + 159
    for (let i = 0; i < name.length; i++) {
        ccolor[i % 3] += name.charCodeAt(i);
    }
    ccolor[0] = colCalc(ccolor[0]);
    ccolor[1] = colCalc(ccolor[1]);
    ccolor[2] = colCalc(ccolor[2]);

    return "#" + ccolor.map((v) => v.toString(16).padStart(2, "0")).join("");
}

export interface IUpdateCallback {
    setUpdateCallback(track: () => void, trigger: () => void): void;
}

export function CustomReactive<T extends IUpdateCallback>(value: T): T {
    const state = customRef<T>((track, trigger) => {
        value.setUpdateCallback(track, trigger);

        return {
            get() {
                track();
                return value;
            },
            set(newValue: T) {
                value = newValue;
                trigger();
            },
        };
    });

    return state.value;
}

export function prettyEntityName(
    entity: BaseActor | undefined,
    raceNameMap: Ref<Record<number, string>>,
): string | undefined {
    if (!entity) {
        return undefined;
    }

    if (ActorManager.pcRaceSet.has(entity.raceId)) {
        return entity.name;
    }

    const raceName =
        raceNameMap.value[entity.raceId] || `unknownRace:${entity.raceId}`;
    if (entity instanceof GroupActor) {
        return raceName;
    }

    // for monster
    if (entity.name[0] >= "0" && entity.name[0] <= "9") {
        return `${raceName} (${entity.name.slice(-4)})`;
    }

    // for pet
    return entity.name;
}
