export interface TeamDpsPlayer {
    entityId: string;
    label: string;
    damages: readonly { At: number; Damage: number }[];
}

export interface TeamDpsPoint {
    x: number;
    y: number;
    custom: { from: number; to: number; damage: number };
}

export function buildTeamDpsTimeline(
    players: readonly TeamDpsPlayer[], startAt: number, endAt: number, requestedStep: number,
) {
    const duration = Number.isFinite(startAt) && Number.isFinite(endAt) ? Math.max(0, endAt - startAt) : 0;
    // Bound the chart size for very long recordings while retaining uniform intervals.
    const stepSeconds = Math.max(Number.isFinite(requestedStep) && requestedStep >= 1 ? requestedStep : 1,
        Math.ceil(duration / 12_000));
    const count = Math.ceil(duration / stepSeconds);
    const emptyPoints = (): TeamDpsPoint[] => [{ x: 0, y: 0, custom: { from: 0, to: 0, damage: 0 } }];
    const totals = new Array<number>(count).fill(0);
    const pointsFromBins = (bins: number[]): TeamDpsPoint[] => {
        const points = emptyPoints();
        for (let index = 0; index < count; index++) {
            const from = index * stepSeconds;
            const to = Math.min(duration, from + stepSeconds);
            points.push({ x: to, y: bins[index] / (to - from), custom: { from, to, damage: bins[index] } });
        }
        return points;
    };
    const members = players.map((player) => {
        const bins = new Array<number>(count).fill(0);
        if (count > 0) {
            for (const event of player.damages) {
                if (!Number.isFinite(event.At) || !Number.isFinite(event.Damage) || event.Damage <= 0
                    || event.At < startAt || event.At > endAt) continue;
                // (from, to], including the opening hit in the first interval.
                // Ownership cannot change when a later hit extends a live battle.
                const index = Math.max(0, Math.min(count - 1, Math.ceil((event.At - startAt) / stepSeconds) - 1));
                bins[index] += event.Damage;
                totals[index] += event.Damage;
            }
        }
        return { entityId: player.entityId, label: player.label, points: pointsFromBins(bins) };
    });
    const total = pointsFromBins(totals);
    const peak = total.reduce((best, point) => point.y > best.y ? point : best, total[0]);
    return { members, total, peak, stepSeconds };
}
