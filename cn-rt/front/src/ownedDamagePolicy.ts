/**
 * Damage emitted by a marionette is credited to its player owner. Normal pet
 * attacks and other player-owned sources remain outside the player's DPS
 * report.
 */
export function isMarionetteDamageSkill(skillId: number): boolean {
    return (
        (skillId >= 54101 && skillId <= 54106)
        || (skillId >= 54151 && skillId <= 54156)
        || (skillId >= 59167 && skillId <= 59169)
    );
}

export function isIncludedPlayerDamage(damage: {
    PetId: string;
    SkillId: number;
}): boolean {
    return !damage.PetId || isMarionetteDamageSkill(damage.SkillId);
}
