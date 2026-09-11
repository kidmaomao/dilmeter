import type { EntityDamage } from '@/eventActor';
import { needCountSkill } from '@/actionCollector';

export function filterByTimeRange(
    damages: EntityDamage[],
    minAt: number | null,
    maxAt: number | null,
): EntityDamage[] {
    if (minAt === null || maxAt === null) return damages;
    return damages.filter(d => d.At >= minAt && d.At <= maxAt);
}

export type GroupedStats = {
    totalDamage: number;
    groupedTotalDamages: Record<string, number>;
    groupedDamages: Record<string, EntityDamage[]>;
    groupedCount: Record<string, number>;
    groupedCriticalCount: Record<string, number>;
    groupedMinDamages: Record<string, number>;
    groupedMaxDamages: Record<string, number>;
};

export function computeGroupedStats(
    damages: EntityDamage[],
    getGroupKey: (d: EntityDamage) => string,
): GroupedStats {
    const groupedTotalDamages: Record<string, number> = {};
    const groupedCount: Record<string, number> = {};
    const groupedCriticalCount: Record<string, number> = {};
    const groupedMinDamages: Record<string, number> = {};
    const groupedMaxDamages: Record<string, number> = {};
    const groupedDamages: Record<string, EntityDamage[]> = {};
    let totalDamage = 0;

    for (const d of damages) {
        const key = getGroupKey(d);
        totalDamage += d.Damage;

        if (!groupedDamages[key]) {
            groupedDamages[key] = [];
            groupedTotalDamages[key] = 0;
            groupedCount[key] = 0;
            groupedCriticalCount[key] = 0;
        }

        groupedDamages[key].push(d);
        groupedTotalDamages[key] += d.Damage;

        if (d.IsDelayed && !needCountSkill[d.SkillId]) {
            continue;
        }

        groupedCount[key]++;
        if (d.IsCritical) {
            groupedCriticalCount[key]++;
        }

        if (!groupedMinDamages[key] || d.Damage < groupedMinDamages[key]) {
            groupedMinDamages[key] = d.Damage;
        }

        if (d.Damage > (groupedMaxDamages[key] ?? 0)) {
            groupedMaxDamages[key] = d.Damage;
        }
    }

    return { totalDamage, groupedTotalDamages, groupedDamages, groupedCount, groupedCriticalCount, groupedMinDamages, groupedMaxDamages };
}
