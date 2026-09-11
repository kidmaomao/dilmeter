export interface PlayerIdentity {
    entityId: string;
}

/**
 * The main report is deliberately local-player-only. Team identity disclosure
 * belongs to the team chart and must never expand this selection.
 */
export function selectMainReportPlayer<T extends PlayerIdentity>(
    players: T[],
    localEntityId: string,
    importedPlayerId = "",
): T | undefined {
    if (localEntityId) return players.find((player) => player.entityId === localEntityId);
    if (importedPlayerId) return players.find((player) => player.entityId === importedPlayerId);
    return undefined;
}
