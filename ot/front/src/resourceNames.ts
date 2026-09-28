import { MabiDB } from "@/mabidb";
import { condNameMap, itemNameMap, raceNameMap, skillNameMap, multiClassNameMap, resourceNameVersion, region, lang } from "@/store";
import { normalizeSkillDisplayName } from "@/skillDisplay";
import { saveResourceRegion, type ResourceRegion } from "@/uiLocale";
import { ref } from "vue";

export const resourceNamesLoading = ref(false);

// Prepare a complete replacement before publishing any selection or names.
// Failed downloads must never leave a mix of CN and TW names on screen.
export async function loadResourceNames(selected: ResourceRegion, refresh = false) {
    if (resourceNamesLoading.value) throw new Error("资料正在载入，请稍候。");
    resourceNamesLoading.value = true;
    const database = new MabiDB(selected, selected);
    try {
        await database.tryOpen();
        if (refresh) await database.forceUpdate();
        const [races, skills, conditions, items, multiClasses] = await Promise.all([
            database.getSortedListData("RaceList"), database.getSortedListData("SkillList"),
            database.getSortedListData("CharCondList"), database.getSortedListData("ItemList"),
            database.getSortedListData("MultiClassList"),
        ]);
        if (!races.length || !skills.length || !conditions.length || (selected === 'tw' && !multiClasses.length)) {
            throw new Error(`${selected.toUpperCase()} 资料不完整，请联网后重试。`);
        }
        const names = <T extends { Id: number; Name: string }>(list: T[], skill = false) =>
            Object.fromEntries(list.map(item => {
                const name = database.getCurLangString(item.Name);
                return [item.Id, skill ? normalizeSkillDisplayName(item.Id, name, selected) : name];
            }));
        const raceNames = Object.fromEntries(races.map(race => [race.Id, `${database.getCurLangString(race.Name)} ${race.Id}`]));
        const skillNames = names(skills, true);
        const conditionNames = names(conditions);
        const itemNames = names(items);
        const multiClassNames = names(multiClasses);
        const version = await database.getDataVersion();
        raceNameMap.value = raceNames;
        skillNameMap.value = skillNames;
        condNameMap.value = conditionNames;
        itemNameMap.value = itemNames;
        multiClassNameMap.value = multiClassNames;
        region.value = selected;
        lang.value = selected;
        saveResourceRegion(selected);
        resourceNameVersion.value++;
        return { version, cached: database.updateError !== null,
            counts: { skills: skills.length, conditions: conditions.length, multiClasses: multiClasses.length } };
    } finally {
        database.close();
        resourceNamesLoading.value = false;
    }
}
