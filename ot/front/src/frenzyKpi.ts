import type { eventBase, eventCharacterConditionEnable, eventArcanaSignal } from "./protocols";

// Frenzy (54402) is CC 230. Skill availability toggles (27044) are not casts.
// Keep the raw state edges alongside other KPI evidence so worker snapshots
// and saved battle records preserve the same complete episodes.
export function recordFrenzySignal(event: eventBase, signals: eventArcanaSignal[]) {
    if (event.EventId !== 4 && event.EventId !== 5 && event.EventId !== 6) return;
    const condition = event as eventCharacterConditionEnable;
    if (event.EventId !== 6 && condition.CCId !== 230) return;
    const exact = Number.isFinite(condition.AtMs) && condition.AtMs! > 0;
    signals.push({ EventId: 22, Id: event.Id, At: event.At,
        AtMs: exact ? condition.AtMs! : event.At * 1000, SkillId: 54402,
        Signal: "frenzy-state", Count: event.EventId === 4 ? 1 : 0,
        Complete: event.EventId !== 4 || (!condition.Snapshot && exact),
        Phase: exact ? 1 : 0,
        ReadyAtMs: event.EventId === 4 ? condition.DisableAtMs || condition.DisableAt * 1000 : undefined,
    });
}

export function frenzyDurations(signals: readonly eventArcanaSignal[], playerId: string, start: number, end: number) {
    const edges = signals.filter((signal) => signal.Id === playerId && signal.SkillId === 54402
        && (signal.Signal === "frenzy-state" || signal.Signal === "frenzy-reset")
        && Number.isFinite(signal.AtMs) && signal.AtMs < end * 1000).sort((a, b) => a.AtMs - b.AtMs);
    const samples: { seconds: number; estimated: boolean }[] = [];
    let active: eventArcanaSignal | undefined;
    let expiry = 0;
    let seen = false;
    for (const edge of edges) {
        if (edge.AtMs >= start * 1000 && edge.Signal === "frenzy-state") seen = true;
        if (edge.Signal === "frenzy-reset") { active = undefined; expiry = 0; continue; }
        if (edge.Count === 1) {
            // Repeated observations/extension of the same active Buff do not
            // restart its duration. A later enable after expiry starts anew.
            if (!active || expiry > 0 && edge.AtMs > expiry + 1000) active = edge;
            expiry = edge.ReadyAtMs ?? 0;
            continue;
        }
        if (edge.Count !== 0) continue;
        if (active?.Complete && active.AtMs >= start * 1000 && edge.AtMs > active.AtMs) {
            // A stale remove must not stretch an episode far beyond its known
            // server lifetime. A one-second tolerance supports legacy logs.
            if (!expiry || edge.AtMs <= expiry + 1000) {
                const seconds = (Math.min(edge.AtMs, expiry || Infinity) - active.AtMs) / 1000;
                if (seconds > 0) samples.push({ seconds, estimated: edge.Phase !== 1 || active.Phase !== 1 });
            }
        }
        active = undefined; expiry = 0;
    }
    return { samples, seen };
}
