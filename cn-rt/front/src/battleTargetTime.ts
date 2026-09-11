export type PositiveDamageTimeRange = {
    startAt: number;
    endAt: number;
};

type DamageTimestamp = {
    At: number;
    Damage: number;
};

/** Return the real first and last positive-damage timestamps, regardless of input order. */
export function buildPositiveDamageTimeRange(
    damages: readonly DamageTimestamp[],
): PositiveDamageTimeRange | null {
    let startAt = Infinity;
    let endAt = -Infinity;

    for (const damage of damages) {
        if (damage.Damage <= 0 || !Number.isFinite(damage.At)) continue;
        startAt = Math.min(startAt, damage.At);
        endAt = Math.max(endAt, damage.At);
    }

    return Number.isFinite(startAt) && Number.isFinite(endAt)
        ? { startAt, endAt }
        : null;
}

export function formatLocalClockTime(timestampSeconds: number): string {
    const date = new Date(timestampSeconds * 1000);
    if (Number.isNaN(date.getTime())) return "--:--:--";

    return [date.getHours(), date.getMinutes(), date.getSeconds()]
        .map((value) => String(value).padStart(2, "0"))
        .join(":");
}

export function formatBattleTargetTimeRange(
    range: PositiveDamageTimeRange | null,
): string {
    if (!range) return "";
    return `${formatLocalClockTime(range.startAt)}~${formatLocalClockTime(range.endAt)}`;
}
