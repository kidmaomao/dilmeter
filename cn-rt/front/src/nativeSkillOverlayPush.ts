import type { SkillCooldownOverlayMessage } from './skillCooldown';

export const NATIVE_SKILL_OVERLAY_EVENT = 'dilmeter-skill-overlay-state';

// Native pushes and HTTP recovery snapshots can arrive out of order, even
// within the same millisecond. Once present, the server revision is canonical.
export function createSkillOverlayMessageGate() {
    let revision = 0;
    let atMs = 0;
    return (message: SkillCooldownOverlayMessage): boolean => {
        if (!message || message.type !== 'skill-cooldown-state' || !Number.isFinite(message.atMs)) return false;
        const nextRevision = message.revision ?? 0;
        if (!Number.isSafeInteger(nextRevision) || nextRevision < 0) return false;
        if (nextRevision > 0) {
            if (nextRevision <= revision) return false;
            revision = nextRevision;
        } else if (revision > 0 || message.atMs < atMs) {
            return false;
        }
        atMs = message.atMs;
        return true;
    };
}

export function subscribeNativeSkillOverlayPush(target: EventTarget, apply: (message: SkillCooldownOverlayMessage) => void): () => void {
    const listener = (event: Event) => {
        const message = (event as CustomEvent<SkillCooldownOverlayMessage>).detail;
        if (message?.type === 'skill-cooldown-state') apply(message);
    };
    target.addEventListener(NATIVE_SKILL_OVERLAY_EVENT, listener);
    return () => target.removeEventListener(NATIVE_SKILL_OVERLAY_EVENT, listener);
}
