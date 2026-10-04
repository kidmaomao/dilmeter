import { condNameMap, multiClassNameMap, skillNameMap } from './store';
import { resourceRegion, uiText } from './uiLocale';
import { TW_GAME_TERMS, type GameTerm } from './data/twGameTerms';

function termName(term: GameTerm): string {
    const names = term.kind === 'skill' ? skillNameMap.value
        : term.kind === 'condition' ? condNameMap.value : multiClassNameMap.value;
    return names[term.id]?.trim() || term.tw;
}

export function skillResourceName(id: number, fallback = ''): string {
    if (resourceRegion.value !== 'tw') return uiText(fallback || skillNameMap.value[id] || `技能 ${id}`);
    return skillNameMap.value[id]?.trim()
        || TW_GAME_TERMS.find(term => term.kind === 'skill' && term.id === id)?.tw
        || `技能 ${id}`;
}

const aliases = TW_GAME_TERMS.flatMap(term => term.aliases.map(alias => ({ alias, term })))
    .sort((a, b) => b.alias.length - a.alias.length);
const byAlias = new Map(aliases.map(entry => [entry.alias, entry.term]));
const pattern = new RegExp(aliases.map(({ alias }) => alias.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|'), 'g');

// Use only for authored interface copy. Never pass player names, editable text,
// filenames or detection/configuration keys through the terminology formatter.
// Resolve by server first; script conversion must not rewrite official names.
export function gameUiText(value: unknown): string {
    const text = value == null ? '' : String(value);
    if (resourceRegion.value !== 'tw') return uiText(text);
    const names: string[] = [];
    const protectedText = text.replace(pattern, alias => {
        const index = names.push(termName(byAlias.get(alias)!)) - 1;
        return `\uE000${index}\uE001`;
    }).replace(/多尔卡/g, '黑闇值').replace(/阿尔卡纳职业|阿尔卡纳/g, '秘法才能');
    return uiText(protectedText).replace(/\uE000(\d+)\uE001/g, (_, index) => names[Number(index)]);
}
