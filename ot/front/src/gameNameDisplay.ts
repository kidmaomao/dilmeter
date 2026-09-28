import { condNameMap, multiClassNameMap } from './store';
import { normalizeNameSearch, resourceRegion, uiText } from './uiLocale';

// Detection/configuration keys remain stable. Only presentation follows the
// selected server, so existing reports, privacy labels and saved rules still work.
const multiClassAliases: Record<string, number> = {
    '元素骑士': 1, '圣光颂唱者': 2, '黑魔导士': 3,
    '流星射手': 4, '流星弓手': 4, '圣盾骑士': 5, '爆裂骑士枪': 6,
    '枪炮师': 7, '禁术炼金师': 8, '禁术炼金术师': 8,
    '旋律操纵师': 9, '狂怒斗士': 10,
};

export function jobDisplayName(name: string | null | undefined): string {
    if (!name) return '';
    if (resourceRegion.value !== 'tw') return uiText(name);
    const id = multiClassAliases[normalizeNameSearch(name)];
    return (id && multiClassNameMap.value[id]) || name;
}

export function jobDetailText(text: string | undefined): string | undefined {
    if (!text || resourceRegion.value !== 'tw') return text;
    return text.replace(/[^/；]+/g, part => {
        const name = part.trim();
        return part.replace(name, jobDisplayName(name));
    });
}

export function conditionResourceName(id: number, fallback = ''): string {
    return resourceRegion.value === 'tw'
        ? condNameMap.value[id]?.trim() || fallback
        : fallback || condNameMap.value[id]?.trim() || '';
}

// Build search options with the same server precedence as reminder labels.
// Defaults add categories/details; they must not hide TW resource names (CC 476).
export function conditionDefinitions<T extends { id: number; name: string }>(
    defaults: T[], fallback: Record<string, string>, resources: Record<number, string>,
    selected: string, hidden: ReadonlySet<number> = new Set(),
): Array<T | { id: number; name: string }> {
    const definitions = new Map<number, T | { id: number; name: string }>(defaults.map(item => [item.id, { ...item }]));
    const defaultIds = new Set(definitions.keys());
    for (const [rawId, name] of Object.entries(fallback)) {
        const id = Number(rawId);
        if (!definitions.has(id)) definitions.set(id, { id, name });
    }
    for (const [rawId, name] of Object.entries(resources)) {
        const id = Number(rawId);
        if (selected === 'tw' || !defaultIds.has(id)) definitions.set(id, { ...definitions.get(id), id, name });
    }
    return [...definitions.values()].filter(item => Number.isInteger(item.id) && item.id >= 0 && !hidden.has(item.id))
        .sort((a, b) => a.id - b.id);
}

export function arcanaLabel(): string {
    return resourceRegion.value === 'tw' ? '秘法才能' : uiText('阿尔卡纳职业');
}
