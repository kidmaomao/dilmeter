import { resDataCall } from "@/lib/apicall";
import {
    condNameMap,
    raceNameMap,
    resourceNameVersion,
    skillNameMap,
} from "@/store";
import { normalizeSkillDisplayName } from "@/skillDisplay";

let loadPromise: Promise<void> | null = null;

export function ensureCnResourceNames(): Promise<void> {
    if (!loadPromise) {
        loadPromise = loadCnResourceNames();
    }
    return loadPromise;
}

async function loadCnResourceNames(): Promise<void> {
    const data = await resDataCall("resourcedata/cn/cn_resourcedata.bin.br", {
        reload: true,
    });
    const strings = new Map(data.StringTable.map((s) => [s.Id, s.Str]));

    const skills: Record<number, string> = {};
    for (const skill of data.SkillList) {
        const name = strings.get(skill.Name) || skill.Name;
        if (name && name !== "None") {
            skills[skill.Id] = normalizeSkillDisplayName(skill.Id, name);
        }
    }

    const conds: Record<number, string> = {};
    for (const cond of data.CharCondList) {
        const name = strings.get(cond.Name) || cond.Name;
        if (name && name !== "None") conds[cond.Id] = name;
    }

    const races: Record<number, string> = {};
    for (const race of data.RaceList) {
        const name = strings.get(race.Name) || race.Name;
        if (name && name !== "None") races[race.Id] = `${name} ${race.Id}`;
    }

    skillNameMap.value = { ...skillNameMap.value, ...skills };
    condNameMap.value = { ...condNameMap.value, ...conds };
    raceNameMap.value = { ...raceNameMap.value, ...races };
    resourceNameVersion.value++;

    console.info(
        `Loaded embedded CN names: ${Object.keys(skills).length} skills, ${Object.keys(conds).length} conditions, ${Object.keys(races).length} races`,
    );
}
