import type * as protocols from "@/protocols";

/** Public health attributes emitted by the game server. */
export const CURRENT_HEALTH_STAT_ID = 28;
export const MAXIMUM_HEALTH_STAT_ID = 30;

/** One authoritative decrease of an entity's server-reported health bar. */
export type EntityHealthLoss = {
    Id: string;
    At: number;
    Damage: number;
};

export type EffectiveDamageAllocation<T extends protocols.eventDamage = protocols.eventDamage> = {
    event: T;
    damage: number;
};

/**
 * The health attributes are float32 values.  At Boss-sized values, a one-point
 * hit can therefore move the exposed health by 64/128/256 rather than by the
 * packet's exact damage.  Two ULPs are enough to absorb that representation
 * error without turning a genuinely immune hit into effective damage.
 */
export function float32HealthTolerance(value: number): number {
    const magnitude = Math.abs(value);
    if (!Number.isFinite(magnitude) || magnitude === 0) return 2;
    const exponent = Math.floor(Math.log2(magnitude));
    return Math.max(2, 2 ** Math.max(-149, exponent - 22));
}

/**
 * Reconciles raw damage packets with one authoritative Boss-health decrease.
 *
 * Damage accumulated while the health value is unchanged can include immune
 * hits.  The server only tells us the aggregate health decrease, so the most
 * recent packets consume that decrease first.  This matches packet ordering
 * at phase thresholds and clips the final overkill.  When truly simultaneous
 * players land in one update, attribution necessarily follows packet order;
 * the aggregate remains exact.
 */
export function allocateEffectiveDamage<T extends protocols.eventDamage>(
    pending: readonly T[],
    previousHealth: number,
    nextHealth: number,
): EffectiveDamageAllocation<T>[] {
    if (
        !Number.isFinite(previousHealth)
        || !Number.isFinite(nextHealth)
        || nextHealth >= previousHealth
    ) {
        return [];
    }

    // Stat28 may continue below zero for packets that land immediately after
    // the first finishing hit. The signed decrease confirms those packets for
    // DPS attribution; authoritative Boss-health loss is clamped separately
    // when EntityHealthLoss is recorded.
    let remaining = previousHealth - nextHealth;
    const tolerance = float32HealthTolerance(Math.max(previousHealth, nextHealth));
    const allocations: EffectiveDamageAllocation<T>[] = [];

    for (let index = pending.length - 1; index >= 0 && remaining > 0; index--) {
        const event = pending[index];
        const rawDamage = Number(event.Damage);
        if (!Number.isFinite(rawDamage) || rawDamage <= 0) continue;

        const damage = Math.min(rawDamage, remaining);
        if (damage > 0) allocations.push({ event, damage });
        remaining -= damage;
    }

    // A float32 health delta can exceed the exact packet sum by a tiny amount.
    // Assign only that representational residue to the most recent hit so the
    // displayed effective total remains equal to the real health-bar decrease.
    if (remaining > 0 && remaining <= tolerance) {
        if (allocations.length > 0) {
            allocations[0].damage += remaining;
        } else if (pending.length > 0) {
            allocations.push({ event: pending[pending.length - 1], damage: remaining });
        }
        remaining = 0;
    }

    return allocations.reverse();
}
