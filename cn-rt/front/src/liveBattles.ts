import type { eventBase } from "./protocols";

export interface LiveBattleTarget {
    sessionKey: string;
    entityId: string;
    name: string;
    raceId: number;
    startedAt: number;
    endedAt: number;
    totalDamage: number;
    maximumHealth: number;
}

export interface LiveBattleCatalog {
    currentSessionKey: string;
    sequence: number;
    targets: LiveBattleTarget[];
}

/** Sequence numbers preserve identical rapid multihits while excluding the
 * overlap between a disk snapshot, a reconnect, and the buffered live tail. */
export class LiveBattleCursor {
    sessionKey = "";
    sequence = 0;

    accept(event: eventBase): boolean {
        if (event.EventId < 0) return true; // transient messages are not written to the log
        const sequence = Number(event.Sequence) || 0;
        if (sequence === 0) return true; // connection bootstrap / legacy imports
        if (sequence <= this.sequence) return false;
        this.sequence = sequence;
        return true;
    }

    requestUrl(incremental: boolean): string {
        const query = new URLSearchParams({ session: "latest" });
        if (this.sessionKey) {
            query.set("knownSession", this.sessionKey);
            query.set("minimumSequence", String(this.sequence));
        }
        if (incremental && this.sessionKey) {
            query.set("afterSequence", String(this.sequence));
        }
        return `/api/live_battles?${query}`;
    }
}

export function needsLiveRecovery(suspended: boolean, connected: boolean): boolean {
    // A focus change or a delayed UI timer alone is not evidence of lost data.
    return suspended || !connected;
}

export async function applyLiveDelta(
    ndjson: string,
    apply: (events: eventBase[]) => void,
    yieldToUi: () => Promise<void>,
): Promise<void> {
    let position = 0;
    let batch: eventBase[] = [];
    while (position < ndjson.length) {
        const newline = ndjson.indexOf("\n", position);
        const end = newline < 0 ? ndjson.length : newline;
        const line = ndjson.slice(position, end).trim();
        position = end + 1;
        if (line) {
            const event = JSON.parse(line) as eventBase;
            // A sparse read may include the writer's initial connection
            // bootstrap. Reapplying that stale state would resurrect actors.
            if (Number(event.Sequence) > 0) batch.push(event);
        }
        if (batch.length >= 512) {
            apply(batch);
            batch = [];
            await yieldToUi();
        }
    }
    if (batch.length) apply(batch);
}
