export interface BattleTargetSelectionItem {
    entityId: string;
    active?: boolean;
}

export interface BattleTargetSelectionState {
    selectedId: string;
    manuallyLockedId: string;
}

/** Reconcile removed targets without turning an automatic choice into a lock. */
export function reconcileBattleTargetSelection(
    state: BattleTargetSelectionState,
    items: readonly BattleTargetSelectionItem[],
): BattleTargetSelectionState {
    const itemIds = new Set(items.map((item) => item.entityId));
    if (state.manuallyLockedId && itemIds.has(state.manuallyLockedId)) {
        return {
            selectedId: state.manuallyLockedId,
            manuallyLockedId: state.manuallyLockedId,
        };
    }
    const selectedId = state.selectedId && itemIds.has(state.selectedId)
        ? state.selectedId
        : items.find((item) => item.active)?.entityId ?? items[0]?.entityId ?? "";
    return { selectedId, manuallyLockedId: "" };
}

/** Only an explicit selection from the target picker establishes a lock. */
export function lockBattleTargetSelection(targetId: string): BattleTargetSelectionState {
    return { selectedId: targetId, manuallyLockedId: targetId };
}

/** Automatic mode follows new damage only after the displayed target is no longer active. */
export function selectDamageBattleTarget(
    state: BattleTargetSelectionState,
    damagedTargetId: string,
    currentTargetIsActive: boolean,
): BattleTargetSelectionState {
    if (state.manuallyLockedId || (state.selectedId && currentTargetIsActive)) return state;
    return { selectedId: damagedTargetId, manuallyLockedId: "" };
}
