import type { eventArcanaSignal } from "./protocols";

type Interval = { start: number; end: number };
type Node = { at: number; lower: number; upper: number; delta: number };
type Segment = { nodes: Node[]; rate: number; maximum: number; end: number };

// CN test parameters confirmed by the user. Unknown initial energy remains
// a range, never an invented zero. Actual signed deltas already include any
// cap clipping, so they also constrain the energy immediately before them.
export function estimateFighterIdle(signals: readonly eventArcanaSignal[], eligible: readonly Interval[], end: number) {
    const segments: Segment[] = [];
    let segment: Segment | undefined;
    let inferred = false;
    const close = (at: number) => { if (segment) segment.end = Math.min(end, at); segment = undefined; };
    for (const signal of signals) {
        const at = signal.AtMs / 1000;
        if (at > end) break;
        if (signal.Signal === "fighter-energy-reset") { close(at); continue; }
        if (signal.Signal === "fighter-energy-baseline" || signal.Signal === "fighter-energy-bounds") {
            close(at);
            const lower = signal.Value ?? 0;
            const upper = signal.Signal === "fighter-energy-bounds" ? signal.UpperValue ?? 0 : lower;
            const rate = signal.Rate ?? 0, maximum = signal.Maximum ?? 0;
            if (![lower, upper, rate, maximum].every(Number.isFinite) || lower < 0 || upper < lower || upper > maximum || maximum < 100 || rate < 0) return;
            segment = { nodes: [{ at, lower, upper, delta: 0 }], rate, maximum, end };
            segments.push(segment);
            inferred ||= signal.Signal === "fighter-energy-bounds";
            continue;
        }
        if (signal.Signal !== "fighter-energy-delta") continue;
        const delta = signal.Value ?? 0;
        if (!Number.isFinite(delta)) return;
        if (!segment) {
            inferred = true;
            segment = { nodes: [{ at, lower: 0, upper: 400, delta: 0 }], rate: 3, maximum: 400, end };
            segments.push(segment);
        }
        const previous = segment.nodes.at(-1)!;
        const growth = (at - previous.at) * segment.rate;
        const lower = Math.max(Math.min(segment.maximum, previous.lower + growth), -delta);
        const upper = Math.min(Math.min(segment.maximum, previous.upper + growth), segment.maximum - delta);
        if (at < previous.at || lower > upper + .1) return;
        segment.nodes.push({ at, lower: Math.max(0, Math.min(lower, upper) + delta), upper: Math.min(segment.maximum, upper + delta), delta });
    }
    if (!inferred) return;
    const overlap = (start: number, finish: number) => eligible.reduce((sum, window) => sum + Math.max(0, Math.min(finish, window.end) - Math.max(start, window.start)), 0);
    let lowerSeconds = 0, upperSeconds = 0, covered = 0;
    for (const group of segments) {
        // A later successful spend proves sufficient energy beforehand.
        // Propagate that constraint backwards without crossing resets.
        for (let i = group.nodes.length - 1; i > 0; i--) {
            const current = group.nodes[i], previous = group.nodes[i - 1];
            const growth = (current.at - previous.at) * group.rate;
            previous.lower = Math.max(previous.lower, current.lower - current.delta - growth);
            const beforeUpper = current.upper - current.delta;
            previous.upper = Math.min(previous.upper, beforeUpper >= group.maximum - .0001 ? group.maximum : beforeUpper - growth);
            if (previous.lower > previous.upper + .1) return;
            previous.lower = Math.min(previous.lower, previous.upper);
        }
        for (let i = 0; i < group.nodes.length; i++) {
            const node = group.nodes[i], until = group.nodes[i + 1]?.at ?? group.end;
            const ready = (value: number) => value >= 100 ? node.at : group.rate > 0 ? node.at + (100 - value) / group.rate : Infinity;
            lowerSeconds += overlap(ready(node.lower), until);
            upperSeconds += overlap(ready(node.upper), until);
            covered += overlap(node.at, until);
        }
    }
    const required = eligible.reduce((sum, window) => sum + window.end - window.start, 0);
    // Before the first observation or during a reset gap, either state is
    // possible; count that time only in the upper bound.
    const unknownSeconds = Math.max(0, required - covered);
    return { lower: lowerSeconds, upper: Math.min(required, upperSeconds + unknownSeconds), unknownSeconds };
}
