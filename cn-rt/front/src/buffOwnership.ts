export type LocalBuffCondition = {
    At: number;
};

export type LocalBuffActor<TCondition extends LocalBuffCondition> = {
    isPC: boolean;
    conditionMap: Record<number, TCondition>;
};

export type LocalBuffManager<TCondition extends LocalBuffCondition> = {
    localEntityId: string;
    localEntityReliable: boolean;
    entityMap: Record<string, LocalBuffActor<TCondition>>;
    pendingConditionMap: Record<string, Record<number, TCondition>>;
};

export type LocalBuffMatch<TCondition extends LocalBuffCondition> = {
    actor: LocalBuffActor<TCondition> | null;
    condition: TCondition;
};

/**
 * Returns a Buff only when its target is the exact entity id reported by the
 * client's private-stat stream. CN does not consistently replay the local
 * player's EntityAppear packet, so that id can remain provisional even though
 * it already points at the player. When entity data exists we still require a
 * PC actor; an identified pet can therefore never drive this client's alert.
 * Attacker/source ownership is intentionally irrelevant because a party
 * member or pet may legitimately apply a Buff to the local player.
 */
export function findLocalBuffCondition<TCondition extends LocalBuffCondition>(
    manager: LocalBuffManager<TCondition>,
    ccId: number,
): LocalBuffMatch<TCondition> | null {
    if (!manager.localEntityId) return null;

    const actor = manager.entityMap[manager.localEntityId];
    if (actor) {
        if (!actor.isPC) return null;
        const condition = actor.conditionMap[ccId];
        return condition ? { actor, condition } : null;
    }

    // When Dilmeter starts mid-map, CN can send the player's private stat and
    // condition packets without repeating that player's EntityAppear packet.
    // Use only this exact pending id; never scan other actors or owned pets.
    const condition = manager.pendingConditionMap[manager.localEntityId]?.[ccId];
    return condition ? { actor: null, condition } : null;
}
